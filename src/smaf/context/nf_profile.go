package context

import (
	"fmt"
	"free5gc/lib/openapi/models"
	"free5gc/src/smaf/factory"
	"time"
)

var NFServices *[]models.NfService

var NfServiceVersion *[]models.NfServiceVersion

var SmafInfo *models.SmfInfo

func SetupNFProfile(config *factory.Config) {
	//Set time
	nfSetupTime := time.Now()

	//set NfServiceVersion
	NfServiceVersion = &[]models.NfServiceVersion{
		{
			ApiVersionInUri: "v1",
			ApiFullVersion:  fmt.Sprintf("https://%s:%d/nsmf-pdusession/v1", SMAF_Self().RegisterIPv4, SMAF_Self().SBIPort),
			Expiry:          &nfSetupTime,
		},
	}

	//set NFServices
	NFServices = new([]models.NfService)
	for _, serviceName := range config.Configuration.ServiceNameList {
		if serviceName == "nausf-auth" {
			continue
		} else {
			*NFServices = append(*NFServices, models.NfService{
				ServiceInstanceId: SMAF_Self().NfInstanceID + serviceName,
				ServiceName:       models.ServiceName(serviceName),
				Versions:          NfServiceVersion,
				Scheme:            models.UriScheme_HTTPS,
				NfServiceStatus:   models.NfServiceStatus_REGISTERED,
				ApiPrefix:         fmt.Sprintf("%s://%s:%d", SMAF_Self().UriScheme, SMAF_Self().RegisterIPv4, SMAF_Self().SBIPort),
			})
		}
		/*
			*NFServices = append(*NFServices, models.NfService{
				ServiceInstanceId: SMAF_Self().NfInstanceID + serviceName,
				ServiceName:       models.ServiceName(serviceName),
				Versions:          NfServiceVersion,
				Scheme:            models.UriScheme_HTTPS,
				NfServiceStatus:   models.NfServiceStatus_REGISTERED,
				ApiPrefix:         fmt.Sprintf("%s://%s:%d", SMAF_Self().UriScheme, SMAF_Self().RegisterIPv4, SMAF_Self().SBIPort),
			})
		*/
	}
	fmt.Println("show smaf NFServices:", NFServices)
	//set smfInfo
	SmafInfo = &models.SmfInfo{
		SNssaiSmfInfoList: &smafContext.SnssaiInfos,
	}
}
