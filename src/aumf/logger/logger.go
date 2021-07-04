package logger

import (
	"os"
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"free5gc/lib/logger_conf"
	"free5gc/lib/logger_util"
)

var log *logrus.Logger
var AppLog *logrus.Entry
var InitLog *logrus.Entry
var ContextLog *logrus.Entry
var NgapLog *logrus.Entry
var HandlerLog *logrus.Entry
var HttpLog *logrus.Entry
var GmmLog *logrus.Entry
var MtLog *logrus.Entry
var ProducerLog *logrus.Entry
var LocationLog *logrus.Entry
var CommLog *logrus.Entry
var CallbackLog *logrus.Entry
var UtilLog *logrus.Entry
var NasLog *logrus.Entry
var ConsumerLog *logrus.Entry
var EeLog *logrus.Entry
var GinLog *logrus.Entry

//20210602 add ausf log entry
var UeAuthPostLog *logrus.Entry
var Auth5gAkaComfirmLog *logrus.Entry
var EapAuthComfirmLog *logrus.Entry
var AusfHandlerLog *logrus.Entry

func init() {
	log = logrus.New()
	log.SetReportCaller(false)

	log.Formatter = &formatter.Formatter{
		TimestampFormat: time.RFC3339,
		TrimMessages:    true,
		NoFieldsSpace:   true,
		HideKeys:        true,
		FieldsOrder:     []string{"component", "category"},
	}

	free5gcLogHook, err := logger_util.NewFileHook(logger_conf.Free5gcLogFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		log.Hooks.Add(free5gcLogHook)
	}

	selfLogHook, err := logger_util.NewFileHook(logger_conf.NfLogDir+"aumf.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		log.Hooks.Add(selfLogHook)
	}

	AppLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "App"})
	InitLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Init"})
	ContextLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Context"})
	NgapLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "NGAP"})
	HandlerLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Handler"})
	HttpLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "HTTP"})
	GmmLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Gmm"})
	MtLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "MT"})
	ProducerLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Producer"})
	LocationLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "LocInfo"})
	CommLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Comm"})
	CallbackLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Callback"})
	UtilLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Util"})
	NasLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "NAS"})
	ConsumerLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "Consumer"})
	EeLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "EventExposure"})
	GinLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "GIN"})
	//20210704 add ausf log format
	UeAuthPostLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "UeAuthPost"})
	Auth5gAkaComfirmLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "5gAkaAuth"})
	EapAuthComfirmLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "EapAkaAuth"})
	AusfHandlerLog = log.WithFields(logrus.Fields{"component": "AUMF", "category": "AusfHandler"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(bool bool) {
	log.SetReportCaller(bool)
}
