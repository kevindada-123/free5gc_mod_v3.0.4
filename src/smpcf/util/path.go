//+build !debug

package util

import (
	"free5gc/lib/path_util"
)

var SmpcfLogPath = path_util.Gofree5gcPath("free5gc/smpcfsslkey.log")
var SmpcfPemPath = path_util.Gofree5gcPath("free5gc/support/TLS/smpcf.pem")
var SmpcfKeyPath = path_util.Gofree5gcPath("free5gc/support/TLS/smpcf.key")
var DefaultSmpcfConfigPath = path_util.Gofree5gcPath("free5gc/config/smpcfcfg.conf")
