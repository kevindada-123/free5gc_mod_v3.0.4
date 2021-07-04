package util

import (
	"fmt"
	"os"

	"github.com/google/uuid"

	"free5gc/lib/nas/security"
	"free5gc/lib/openapi/models"
	"free5gc/src/aumf/context"
	"free5gc/src/aumf/factory"
	"free5gc/src/aumf/logger"

	"strconv"
)

func InitAmfContext(aumfcontext *context.AUMFContext) {
	config := factory.AmfConfig
	logger.UtilLog.Infof("aumfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	aumfcontext.NfId = uuid.New().String()
	if configuration.AmfName != "" {
		aumfcontext.Name = configuration.AmfName
	}
	if configuration.NgapIpList != nil {
		aumfcontext.NgapIpList = configuration.NgapIpList
	} else {
		aumfcontext.NgapIpList = []string{"127.0.0.1"} // default localhost
	}
	sbi := configuration.Sbi
	if sbi.Scheme != "" {
		aumfcontext.UriScheme = models.UriScheme(sbi.Scheme)
	} else {
		logger.UtilLog.Warnln("SBI Scheme has not been set. Using http as default")
		aumfcontext.UriScheme = "http"
	}
	aumfcontext.RegisterIPv4 = "127.0.0.1" // default localhost
	aumfcontext.SBIPort = 29518            // default port
	if sbi != nil {
		if sbi.RegisterIPv4 != "" {
			aumfcontext.RegisterIPv4 = sbi.RegisterIPv4
		}
		if sbi.Port != 0 {
			aumfcontext.SBIPort = sbi.Port
		}
		aumfcontext.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
		if aumfcontext.BindingIPv4 != "" {
			logger.UtilLog.Info("Parsing ServerIPv4 address from ENV Variable.")
		} else {
			aumfcontext.BindingIPv4 = sbi.BindingIPv4
			if aumfcontext.BindingIPv4 == "" {
				logger.UtilLog.Warn("Error parsing ServerIPv4 address from string. Using the 0.0.0.0 as default.")
				aumfcontext.BindingIPv4 = "0.0.0.0"
			}
		}
	}
	//20210704 added ausf context initialize
	aumfcontext.Url = string(aumfcontext.UriScheme) + "://" + aumfcontext.RegisterIPv4 + ":" + strconv.Itoa(aumfcontext.SBIPort)
	//aumfcontext.PlmnList = append(aumfcontext.PlmnList, configuration.PlmnSupportList...)

	serviceNameList := configuration.ServiceNameList
	aumfcontext.InitNFService(serviceNameList, config.Info.Version)
	aumfcontext.ServedGuamiList = configuration.ServedGumaiList
	aumfcontext.SupportTaiLists = configuration.SupportTAIList
	for i := range aumfcontext.SupportTaiLists {
		aumfcontext.SupportTaiLists[i].Tac = TACConfigToModels(aumfcontext.SupportTaiLists[i].Tac)
	}
	aumfcontext.PlmnSupportList = configuration.PlmnSupportList
	aumfcontext.SupportDnnLists = configuration.SupportDnnList
	if configuration.NrfUri != "" {
		aumfcontext.NrfUri = configuration.NrfUri
	} else {
		logger.UtilLog.Warn("NRF Uri is empty! Using localhost as NRF IPv4 address.")
		aumfcontext.NrfUri = fmt.Sprintf("%s://%s:%d", aumfcontext.UriScheme, "127.0.0.1", 29510)
	}
	security := configuration.Security
	if security != nil {
		aumfcontext.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
		aumfcontext.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	}
	aumfcontext.NetworkName = configuration.NetworkName
	aumfcontext.T3502Value = configuration.T3502
	aumfcontext.T3512Value = configuration.T3512
	aumfcontext.Non3gppDeregistrationTimerValue = configuration.Non3gppDeregistrationTimer
}

func getIntAlgOrder(integrityOrder []string) (intOrder []uint8) {
	for _, intAlg := range integrityOrder {
		switch intAlg {
		case "NIA0":
			intOrder = append(intOrder, security.AlgIntegrity128NIA0)
		case "NIA1":
			intOrder = append(intOrder, security.AlgIntegrity128NIA1)
		case "NIA2":
			intOrder = append(intOrder, security.AlgIntegrity128NIA2)
		case "NIA3":
			intOrder = append(intOrder, security.AlgIntegrity128NIA3)
		default:
			logger.UtilLog.Errorf("Unsupported algorithm: %s", intAlg)
		}
	}
	return
}
func getEncAlgOrder(cipheringOrder []string) (encOrder []uint8) {
	for _, encAlg := range cipheringOrder {
		switch encAlg {
		case "NEA0":
			encOrder = append(encOrder, security.AlgCiphering128NEA0)
		case "NEA1":
			encOrder = append(encOrder, security.AlgCiphering128NEA1)
		case "NEA2":
			encOrder = append(encOrder, security.AlgCiphering128NEA2)
		case "NEA3":
			encOrder = append(encOrder, security.AlgCiphering128NEA3)
		default:
			logger.UtilLog.Errorf("Unsupported algorithm: %s", encAlg)
		}
	}
	return
}
