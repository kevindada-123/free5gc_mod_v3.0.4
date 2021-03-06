package pdusession

import (
	"free5gc/lib/http2_util"
	"free5gc/lib/logger_util"
	"free5gc/lib/path_util"
	"free5gc/src/smaf/logger"
	"free5gc/src/smaf/pfcp"
	"free5gc/src/smaf/pfcp/udp"
	"log"
	"net/http"
)

func DummyServer() {
	router := logger_util.NewGinWithLogrus(logger.GinLog)

	AddService(router)

	go udp.Run(pfcp.Dispatch)

	smfKeyLogPath := path_util.Gofree5gcPath("free5gc/smfsslkey.log")
	SmafPemPath := path_util.Gofree5gcPath("free5gc/support/TLS/smf.pem")
	SmafKeyPath := path_util.Gofree5gcPath("free5gc/support/TLS/smf.key")

	var server *http.Server
	if srv, err := http2_util.NewServer(":29502", smfKeyLogPath, router); err != nil {
	} else {
		server = srv
	}

	if err := server.ListenAndServeTLS(SmafPemPath, SmafKeyPath); err != nil {
		log.Fatal(err)
	}

}
