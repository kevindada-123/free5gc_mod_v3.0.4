/*
 * Nsmf_PDUSession
 *
 * SMF PDU Session Service
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package main

import (
	"fmt"
	"free5gc/src/app"
	"free5gc/src/smpcf/logger"
	"free5gc/src/smpcf/service"
	"free5gc/src/smpcf/version"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var SMPCF = &service.SMPCF{}

var appLog *logrus.Entry

func init() {
	appLog = logger.AppLog
}

func main() {
	app := cli.NewApp()
	app.Name = "smpcf"
	fmt.Print(app.Name, "\n")
	appLog.Infoln("SMPCF version: ", version.GetVersion())
	app.Usage = "-free5gccfg common configuration file -smpcfcfg smpcf configuration file"
	app.Action = action
	app.Flags = SMPCF.GetCliCmd()

	if err := app.Run(os.Args); err != nil {
		logger.AppLog.Errorf("SMPCF Run error: %v", err)
	}
}

func action(c *cli.Context) {
	app.AppInitializeWillInitialize(c.String("free5gccfg"))
	SMPCF.Initialize(c)
	SMPCF.Start()
}