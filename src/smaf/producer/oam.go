package producer

import (
	"free5gc/lib/http_wrapper"
	"free5gc/lib/openapi/models"
	"free5gc/src/smaf/context"
	"net/http"
	"strconv"
)

type PDUSessionInfo struct {
	Supi         string
	PDUSessionID string
	Dnn          string
	Sst          string
	Sd           string
	AnType       models.AccessType
	PDUAddress   string
	SessionRule  models.SessionRule
	UpCnxState   models.UpCnxState
	Tunnel       context.UPTunnel
}

func HandleOAMGetUEPDUSessionInfo(smContextRef string) *http_wrapper.Response {
	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {

		httpResponse := &http_wrapper.Response{
			Header: nil,
			Status: http.StatusNotFound,
			Body:   nil,
		}

		return httpResponse
	}

	httpResponse := &http_wrapper.Response{
		Header: nil,
		Status: http.StatusOK,
		Body: PDUSessionInfo{
			Supi:         smContext.Supi,
			PDUSessionID: strconv.Itoa(int(smContext.PDUSessionID)),
			Dnn:          smContext.Dnn,
			Sst:          strconv.Itoa(int(smContext.Snssai.Sst)),
			Sd:           smContext.Snssai.Sd,
			AnType:       smContext.AnType,
			PDUAddress:   smContext.PDUAddress.String(),
			UpCnxState:   smContext.UpCnxState,
			// Tunnel: context.UPTunnel{
			// 	//UpfRoot:  smContext.Tunnel.UpfRoot,
			// 	ULCLRoot: smContext.Tunnel.UpfRoot,
			// },
		},
	}
	return httpResponse

}
