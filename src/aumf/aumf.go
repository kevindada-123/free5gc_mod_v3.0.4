package main

import (
	"free5gc/src/app"
	"free5gc/src/aumf/logger"
	"free5gc/src/aumf/service"
	"free5gc/src/aumf/version"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var AUMF = &service.AUMF{}

var appLog *logrus.Entry

func init() {
	appLog = logger.AppLog
}

func main() {
	app := cli.NewApp()
	app.Name = "aumf"
	appLog.Infoln(app.Name)
	appLog.Infoln("AUMF version: ", version.GetVersion())
	app.Usage = "-free5gccfg common configuration file -aumfcfg aumf configuration file"
	app.Action = action
	app.Flags = AUMF.GetCliCmd()
	if err := app.Run(os.Args); err != nil {
		logger.AppLog.Errorf("AUMF Run error: %v", err)
	}
}

func action(c *cli.Context) {
	app.AppInitializeWillInitialize(c.String("free5gccfg"))
	AUMF.Initialize(c)
	AUMF.Start()
}
