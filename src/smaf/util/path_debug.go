//+build debug

package util

import (
	"free5gc/lib/path_util"
)

var SmafLogPath = path_util.Gofree5gcPath("free5gc/smafsslkey.log")
var SmafPemPath = path_util.Gofree5gcPath("free5gc/support/TLS/_debug.pem")
var SmafKeyPath = path_util.Gofree5gcPath("free5gc/support/TLS/_debug.key")
var DefaultSmafConfigPath = path_util.Gofree5gcPath("free5gc/config/smafcfg.conf")
