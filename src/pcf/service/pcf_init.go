package service

import (
	"bufio"
	"fmt"
	"os/exec"
	"sync"

	"github.com/antihax/optional"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"free5gc/lib/http2_util"
	"free5gc/lib/logger_util"
	"free5gc/lib/openapi/Nnrf_NFDiscovery"
	"free5gc/lib/openapi/models"
	"free5gc/lib/path_util"
	"free5gc/src/app"
	"free5gc/src/pcf/ampolicy"
	"free5gc/src/pcf/bdtpolicy"
	"free5gc/src/pcf/consumer"
	"free5gc/src/pcf/context"
	"free5gc/src/pcf/factory"
	"free5gc/src/pcf/httpcallback"
	"free5gc/src/pcf/logger"
	"free5gc/src/pcf/oam"
	"free5gc/src/pcf/policyauthorization"
	"free5gc/src/pcf/smpolicy"
	"free5gc/src/pcf/uepolicy"
	"free5gc/src/pcf/util"
)

type PCF struct{}

type (
	// Config information.
	Config struct {
		pcfcfg string
	}
)

var config Config

var pcfCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "pcfcfg",
		Usage: "config file",
	},
}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (*PCF) GetCliCmd() (flags []cli.Flag) {
	return pcfCLi
}

func (*PCF) Initialize(c *cli.Context) {

	config = Config{
		pcfcfg: c.String("pcfcfg"),
	}
	if config.pcfcfg != "" {
		factory.InitConfigFactory(config.pcfcfg)
	} else {
		DefaultPcfConfigPath := path_util.Gofree5gcPath("free5gc/config/pcfcfg.conf")
		factory.InitConfigFactory(DefaultPcfConfigPath)
	}

	if app.ContextSelf().Logger.PCF.DebugLevel != "" {
		level, err := logrus.ParseLevel(app.ContextSelf().Logger.PCF.DebugLevel)
		if err != nil {
			initLog.Warnf("Log level [%s] is not valid, set to [info] level", app.ContextSelf().Logger.PCF.DebugLevel)
			logger.SetLogLevel(logrus.InfoLevel)
		} else {
			logger.SetLogLevel(level)
			initLog.Infof("Log level is set to [%s] level", level)
		}
	} else {
		initLog.Infoln("Log level is default set to [info] level")
		logger.SetLogLevel(logrus.InfoLevel)
	}

	logger.SetReportCaller(app.ContextSelf().Logger.PCF.ReportCaller)
}

func (pcf *PCF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range pcf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (pcf *PCF) Start() {
	initLog.Infoln("Server started")
	router := logger_util.NewGinWithLogrus(logger.GinLog)

	bdtpolicy.AddService(router)
	smpolicy.AddService(router)
	ampolicy.AddService(router)
	uepolicy.AddService(router)
	policyauthorization.AddService(router)
	httpcallback.AddService(router)
	oam.AddService(router)
	//this function can be deleted or commented with distrubing test procedure
	/*
		router.Use(cors.New(cors.Config{
			AllowMethods: []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
			AllowHeaders: []string{"Origin", "Content-Length", "Content-Type", "User-Agent",
				"Referrer", "Host", "Token", "X-Requested-With"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			AllowAllOrigins:  true,
			MaxAge:           86400,
		}))
	*/
	//self := context.PCF_Self()
	//context.InitpcfContext(self)
	context.InitpcfContext()
	/*
		profile, err := consumer.BuildNFInstance(self)
		if err != nil {
			initLog.Error("Build PCF Profile Error")
		}

		_, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
		if err != nil {
			initLog.Errorf("PCF register to NRF Error[%s]", err.Error())
		}
	*/
	// 20210618 added
	consumer.SendNFRegistration()

	// subscribe to all Amfs' status change
	amfInfos := consumer.SearchAvailableAMFs(context.PCF_Self().NrfUri, models.ServiceName_NAMF_COMM)
	//fmt.Printf("PCF  subscribe to all Amfs' status change amfInfos %+v: \n", amfInfos)
	var err error
	for _, amfInfo := range amfInfos {
		guamiList := util.GetNotSubscribedGuamis(amfInfo.GuamiList)
		if len(guamiList) == 0 {
			continue
		}
		var problemDetails *models.ProblemDetails
		problemDetails, err = consumer.AmfStatusChangeSubscribe(amfInfo)
		if problemDetails != nil {
			logger.InitLog.Warnf("AMF status subscribe Failed[%+v]", problemDetails)
		} else if err != nil {
			logger.InitLog.Warnf("AMF status subscribe Error[%+v]", err)
		}
	}

	// TODO: subscribe NRF NFstatus

	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NUDR_DR}),
	}
	resp, err := consumer.SendSearchNFInstances(context.PCF_Self().NrfUri, models.NfType_UDR, models.NfType_PCF, param)
	//fmt.Println("PCF  subscribe NRF NFstatus resp : ", resp)
	for _, nfProfile := range resp.NfInstances {
		udruri := util.SearchNFServiceUri(nfProfile, models.ServiceName_NUDR_DR, models.NfServiceStatus_REGISTERED)
		if udruri != "" {
			//fmt.Println("udruri : ", udruri)
			context.PCF_Self().SetDefaultUdrURI(udruri)
			break
		}
	}
	if err != nil {
		initLog.Errorln(err)
	}
	HTTPAddr := fmt.Sprintf("%s:%d", context.PCF_Self().BindingIPv4, context.PCF_Self().SBIPort)
	server, err := http2_util.NewServer(HTTPAddr, util.PCF_LOG_PATH, router)
	if server == nil {
		initLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("Initialize HTTP server: +%v", err)
	}

	serverScheme := factory.PcfConfig.Configuration.Sbi.Scheme
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(util.PCF_PEM_PATH, util.PCF_KEY_PATH)
	}

	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (pcf *PCF) Exec(c *cli.Context) error {
	initLog.Traceln("args:", c.String("pcfcfg"))
	args := pcf.FilterCli(c)
	initLog.Traceln("filter: ", args)
	command := exec.Command("./pcf", args...)

	stdout, err := command.StdoutPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	stderr, err := command.StderrPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	go func() {
		in := bufio.NewScanner(stderr)
		fmt.Println("PCF log start")
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	go func() {
		fmt.Println("PCF start")
		if err = command.Start(); err != nil {
			fmt.Printf("command.Start() error: %v", err)
		}
		fmt.Println("PCF end")
		wg.Done()
	}()

	wg.Wait()

	return err
}
