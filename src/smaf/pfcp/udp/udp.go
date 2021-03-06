package udp

import (
	"net"
	"time"

	"free5gc/lib/pfcp"
	"free5gc/lib/pfcp/pfcpUdp"
	"free5gc/src/smaf/context"
	"free5gc/src/smaf/logger"
)

const MaxPfcpUdpDataSize = 1024

var Server pfcpUdp.PfcpServer

var ServerStartTime time.Time

func Run(Dispatch func(*pfcpUdp.Message)) {
	CPNodeID := context.SMAF_Self().CPNodeID
	Server = pfcpUdp.NewPfcpServer(CPNodeID.ResolveNodeIdToIp().String())

	err := Server.Listen()
	if err != nil {
		logger.PfcpLog.Errorf("Failed to listen: %v", err)
	}
	logger.PfcpLog.Infof("Listen on %s", Server.Conn.LocalAddr().String())

	go func(p *pfcpUdp.PfcpServer) {
		for {
			var pfcpMessage pfcp.Message
			remoteAddr, err := p.ReadFrom(&pfcpMessage)
			if err != nil {
				if err.Error() == "Receive resend PFCP request" {
					logger.PfcpLog.Infoln(err)
				} else {
					logger.PfcpLog.Warnf("Read PFCP error: %v", err)
				}

				continue
			}

			msg := pfcpUdp.NewMessage(remoteAddr, &pfcpMessage)
			go Dispatch(&msg)

		}
	}(&Server)

	ServerStartTime = time.Now()
}

func SendPfcp(msg pfcp.Message, addr *net.UDPAddr) {

	err := Server.WriteTo(msg, addr)
	if err != nil {
		logger.PfcpLog.Infoln("in smaf pfcp udp sendpfcp")
		logger.PfcpLog.Errorf("Failed to send PFCP message: %v", err)
	}

}
