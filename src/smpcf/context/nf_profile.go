package context

import (
	"fmt"
	"free5gc/lib/openapi/models"
	"free5gc/src/smpcf/factory"
	"time"
)

var NFServices *[]models.NfService

var NfServiceVersion *[]models.NfServiceVersion

var SmpcfInfo *models.SmpcfInfo

func SetupNFProfile(config *factory.Config) {
	//Set time
	nfSetupTime := time.Now()

	//set NfServiceVersion
	NfServiceVersion = &[]models.NfServiceVersion{
		{
			ApiVersionInUri: "v1",
			ApiFullVersion:  fmt.Sprintf("https://%s:%d/nsmf-pdusession/v1", SMPCF_Self().RegisterIPv4, SMPCF_Self().SBIPort),
			Expiry:          &nfSetupTime,
		},
	}

	//set NFServices
	NFServices = new([]models.NfService)
	for _, serviceName := range config.Configuration.ServiceNameList {
		/*
			if serviceName == "nausf-auth" {
				continue
			} else {
				*NFServices = append(*NFServices, models.NfService{
					ServiceInstanceId: SMPCF_Self().NfInstanceID + serviceName,
					ServiceName:       models.ServiceName(serviceName),
					Versions:          NfServiceVersion,
					Scheme:            models.UriScheme_HTTPS,
					NfServiceStatus:   models.NfServiceStatus_REGISTERED,
					ApiPrefix:         fmt.Sprintf("%s://%s:%d", SMPCF_Self().UriScheme, SMPCF_Self().RegisterIPv4, SMPCF_Self().SBIPort),
				})
			}
		*/

		*NFServices = append(*NFServices, models.NfService{
			ServiceInstanceId: SMPCF_Self().NfInstanceID + serviceName,
			ServiceName:       models.ServiceName(serviceName),
			Versions:          NfServiceVersion,
			Scheme:            models.UriScheme_HTTPS,
			NfServiceStatus:   models.NfServiceStatus_REGISTERED,
			ApiPrefix:         fmt.Sprintf("%s://%s:%d", SMPCF_Self().UriScheme, SMPCF_Self().RegisterIPv4, SMPCF_Self().SBIPort),
		})

	}
	//fmt.Printf("show smpcf NFServices: %+v\n", NFServices)
	//set smfInfo
	SmpcfInfo = &models.SmpcfInfo{
		SNssaiSmfInfoList: &smpcfContext.SnssaiInfos,
	}
}
