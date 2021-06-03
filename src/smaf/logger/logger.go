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
var GsmLog *logrus.Entry
var PfcpLog *logrus.Entry
var PduSessLog *logrus.Entry
var SMAFContextLog *logrus.Entry
var GinLog *logrus.Entry

//20210602 add ausf log entry
var UeAuthPostLog *logrus.Entry
var Auth5gAkaComfirmLog *logrus.Entry
var EapAuthComfirmLog *logrus.Entry
var AusfHandlerLog *logrus.Entry
var AusfContextLog *logrus.Entry

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

	selfLogHook, err := logger_util.NewFileHook(logger_conf.NfLogDir+"smaf.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		log.Hooks.Add(selfLogHook)
	}

	AppLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "App"})
	InitLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "Init"})
	PfcpLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "Pfcp"})
	PduSessLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "PduSess"})
	GsmLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "GSM"})
	SMAFContextLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "Context"})
	GinLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "GIN"})
	//20210602 add ausf log format
	UeAuthPostLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "UeAuthPost"})
	Auth5gAkaComfirmLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "5gAkaAuth"})
	EapAuthComfirmLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "EapAkaAuth"})
	AusfHandlerLog = log.WithFields(logrus.Fields{"component": "SMAF", "category": "AusfHandler"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(bool bool) {
	log.SetReportCaller(bool)
}
