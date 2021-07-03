//+build debug

package util

import (
	"free5gc/lib/path_util"
)

var SmpcfLogPath = path_util.Gofree5gcPath("free5gc/smpcfsslkey.log")
var SmpcfPemPath = path_util.Gofree5gcPath("free5gc/support/TLS/_debug.pem")
var SmpcfKeyPath = path_util.Gofree5gcPath("free5gc/support/TLS/_debug.key")
var DefaultSmpcfConfigPath = path_util.Gofree5gcPath("free5gc/config/smpcfcfg.conf")
