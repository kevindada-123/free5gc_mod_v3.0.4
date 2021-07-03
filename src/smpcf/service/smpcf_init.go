package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antihax/optional"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"free5gc/lib/http2_util"
	"free5gc/lib/logger_util"
	"free5gc/lib/openapi/Nnrf_NFDiscovery"
	"free5gc/lib/openapi/models"
	"free5gc/lib/path_util"
	"free5gc/src/app"
	"free5gc/src/smpcf/ampolicy"
	"free5gc/src/smpcf/callback"
	"free5gc/src/smpcf/consumer"
	"free5gc/src/smpcf/context"
	"free5gc/src/smpcf/eventexposure"
	"free5gc/src/smpcf/factory"
	"free5gc/src/smpcf/logger"
	"free5gc/src/smpcf/oam"
	"free5gc/src/smpcf/pdusession"
	"free5gc/src/smpcf/pfcp"
	"free5gc/src/smpcf/pfcp/message"
	"free5gc/src/smpcf/pfcp/udp"
	"free5gc/src/smpcf/smpolicy"
	"free5gc/src/smpcf/util"
)

type SMPCF struct{}

type (
	// Config information.
	Config struct {
		smpcfcfg  string
		uerouting string
	}
)

var config Config

var smfCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "smpcfcfg",
		Usage: "config file",
	},
	cli.StringFlag{
		Name:  "uerouting",
		Usage: "config file",
	},
}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (*SMPCF) GetCliCmd() (flags []cli.Flag) {
	return smfCLi
}

func (*SMPCF) Initialize(c *cli.Context) {

	config = Config{
		smpcfcfg:  c.String("smpcfcfg"),
		uerouting: c.String("uerouting"),
	}

	if config.smpcfcfg != "" {
		factory.InitConfigFactory(config.smpcfcfg)
	} else {
		DefaultSmpcfConfigPath := path_util.Gofree5gcPath("free5gc/config/smpcfcfg.conf")
		factory.InitConfigFactory(DefaultSmpcfConfigPath)
	}

	if config.uerouting != "" {
		factory.InitRoutingConfigFactory(config.uerouting)
	} else {
		DefaultUERoutingPath := path_util.Gofree5gcPath("free5gc/config/uerouting.yaml")
		factory.InitRoutingConfigFactory(DefaultUERoutingPath)
	}

	if app.ContextSelf().Logger.SMF.DebugLevel != "" {
		level, err := logrus.ParseLevel(app.ContextSelf().Logger.SMF.DebugLevel)
		if err != nil {
			initLog.Warnf("Log level [%s] is not valid, set to [info] level", app.ContextSelf().Logger.SMF.DebugLevel)
			logger.SetLogLevel(logrus.InfoLevel)
		} else {
			logger.SetLogLevel(level)
			initLog.Infof("Log level is set to [%s] level", level)
		}
	} else {
		initLog.Infoln("Log level is default set to [info] level")
		logger.SetLogLevel(logrus.InfoLevel)
	}
	logger.SetReportCaller(app.ContextSelf().Logger.SMF.ReportCaller)
}

func (smpcf *SMPCF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range smpcf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (smpcf *SMPCF) Start() {
	//20210601 initial smpcf
	context.InitSmpcfContext(&factory.SmpcfConfig)
	//allocate id for each upf
	context.AllocateUPFID()
	context.InitSMFUERouting(&factory.UERoutingConfig)

	initLog.Infoln("Server started")
	//20210601 initial loggger
	router := logger_util.NewGinWithLogrus(logger.GinLog)
	//20210601 send registration msg to NRF
	err := consumer.SendNFRegistration()
	if err != nil {
		retry_err := consumer.RetrySendNFRegistration(10)
		if retry_err != nil {
			logger.InitLog.Errorln(retry_err)
			return
		}
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		smpcf.Terminate()
		os.Exit(0)
	}()
	//start smpcf(SMF) service and network services
	oam.AddService(router)
	callback.AddService(router)

	//start pcf service and network services
	smpolicy.AddService(router)
	ampolicy.AddService(router)

	for _, serviceName := range factory.SmpcfConfig.Configuration.ServiceNameList {
		switch models.ServiceName(serviceName) {
		case models.ServiceName_NSMF_PDUSESSION:
			pdusession.AddService(router)
		case models.ServiceName_NSMF_EVENT_EXPOSURE:
			eventexposure.AddService(router)
		}
	}

	//20210601 run pfcp connection
	udp.Run(pfcp.Dispatch)

	for _, upf := range context.SMPCF_Self().UserPlaneInformation.UPFs {
		logger.AppLog.Infof("Send PFCP Association Request to UPF[%s]\n", upf.NodeID.NodeIdValue)
		message.SendPfcpAssociationSetupRequest(upf.NodeID)
	}

	time.Sleep(1000 * time.Millisecond)

	//20210619 added pcf intial
	// subscribe to all Amfs' status change
	amfInfos := consumer.SearchAvailableAMFs(context.SMPCF_Self().NrfUri, models.ServiceName_NAMF_COMM)
	//fmt.Printf("PCF  subscribe to all Amfs' status change amfInfos %+v: \n", amfInfos)
	//var err error
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
	resp, err := consumer.SendSearchNFInstances(context.SMPCF_Self().NrfUri, models.NfType_UDR, models.NfType_SMPCF, param)
	//fmt.Println("PCF  subscribe NRF NFstatus resp : ", resp)
	for _, nfProfile := range resp.NfInstances {
		udruri := util.SearchNFServiceUri(nfProfile, models.ServiceName_NUDR_DR, models.NfServiceStatus_REGISTERED)
		if udruri != "" {
			//fmt.Println("udruri : ", udruri)
			context.SMPCF_Self().SetDefaultUdrURI(udruri)
			break
		}
	}
	if err != nil {
		initLog.Errorln(err)
	}

	//20210601 initialize http server
	HTTPAddr := fmt.Sprintf("%s:%d", context.SMPCF_Self().BindingIPv4, context.SMPCF_Self().SBIPort)
	server, err := http2_util.NewServer(HTTPAddr, util.SmpcfLogPath, router)
	if server == nil {
		initLog.Error("Initialize HTTP server failed:", err)
		return
	}

	if err != nil {
		initLog.Warnln("Initialize HTTP server:", err)
	}
	//20210601 determine http or https
	serverScheme := factory.SmpcfConfig.Configuration.Sbi.Scheme
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(util.SmpcfPemPath, util.SmpcfKeyPath)
	}

	if err != nil {
		initLog.Fatalln("HTTP server setup failed:", err)
	}
}

func (smpcf *SMPCF) Terminate() {
	logger.InitLog.Infof("Terminating SMPCF...")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("Deregister from NRF successfully")
	}
}

func (smpcf *SMPCF) Exec(c *cli.Context) error {
	return nil
}
