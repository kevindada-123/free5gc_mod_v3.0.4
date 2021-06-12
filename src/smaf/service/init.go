package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"free5gc/lib/http2_util"
	"free5gc/lib/logger_util"
	"free5gc/lib/openapi/models"
	"free5gc/lib/path_util"
	"free5gc/src/app"
	"free5gc/src/smaf/callback"
	"free5gc/src/smaf/consumer"
	"free5gc/src/smaf/context"
	"free5gc/src/smaf/eventexposure"
	"free5gc/src/smaf/factory"
	"free5gc/src/smaf/logger"
	"free5gc/src/smaf/oam"
	"free5gc/src/smaf/pdusession"
	"free5gc/src/smaf/pfcp"
	"free5gc/src/smaf/pfcp/message"
	"free5gc/src/smaf/pfcp/udp"
	"free5gc/src/smaf/ueauthentication"
	"free5gc/src/smaf/util"
)

type SMAF struct{}

type (
	// Config information.
	Config struct {
		smafcfg   string
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
		Name:  "smafcfg",
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

func (*SMAF) GetCliCmd() (flags []cli.Flag) {
	return smfCLi
}

func (*SMAF) Initialize(c *cli.Context) {

	config = Config{
		smafcfg:   c.String("smafcfg"),
		uerouting: c.String("uerouting"),
	}

	if config.smafcfg != "" {
		factory.InitConfigFactory(config.smafcfg)
	} else {
		DefaultSmafConfigPath := path_util.Gofree5gcPath("free5gc/config/smafcfg.conf")
		factory.InitConfigFactory(DefaultSmafConfigPath)
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

func (smaf *SMAF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range smaf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (smaf *SMAF) Start() {
	//20210601 initial ausf
	context.InitSmafContext(&factory.SmafConfig)
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
		smaf.Terminate()
		os.Exit(0)
	}()
	//start smaf(SMF) service and network services
	oam.AddService(router)
	callback.AddService(router)
	for _, serviceName := range factory.SmafConfig.Configuration.ServiceNameList {
		switch models.ServiceName(serviceName) {
		case models.ServiceName_NSMF_PDUSESSION:
			pdusession.AddService(router)
		case models.ServiceName_NSMF_EVENT_EXPOSURE:
			eventexposure.AddService(router)
		}
	}
	//start ausf service and network services
	ueauthentication.AddService(router)
	//20210601 run pfcp connection
	udp.Run(pfcp.Dispatch)

	for _, upf := range context.SMAF_Self().UserPlaneInformation.UPFs {
		logger.AppLog.Infof("Send PFCP Association Request to UPF[%s]\n", upf.NodeID.NodeIdValue)
		message.SendPfcpAssociationSetupRequest(upf.NodeID)
	}

	time.Sleep(1000 * time.Millisecond)
	//20210601 initialize http server
	HTTPAddr := fmt.Sprintf("%s:%d", context.SMAF_Self().BindingIPv4, context.SMAF_Self().SBIPort)
	server, err := http2_util.NewServer(HTTPAddr, util.SmafLogPath, router)
	if server == nil {
		initLog.Error("Initialize HTTP server failed:", err)
		return
	}

	if err != nil {
		initLog.Warnln("Initialize HTTP server:", err)
	}
	//20210601 determine http or https
	serverScheme := factory.SmafConfig.Configuration.Sbi.Scheme
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(util.SmafPemPath, util.SmafKeyPath)
	}

	if err != nil {
		initLog.Fatalln("HTTP server setup failed:", err)
	}

}

func (smaf *SMAF) Terminate() {
	logger.InitLog.Infof("Terminating SMAF...")
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

func (smaf *SMAF) Exec(c *cli.Context) error {
	return nil
}
