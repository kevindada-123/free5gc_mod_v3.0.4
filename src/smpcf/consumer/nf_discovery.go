package consumer

import (
	"context"
	"fmt"
	"free5gc/lib/openapi/Nnrf_NFDiscovery"
	"free5gc/lib/openapi/models"
	pcf_context "free5gc/src/smpcf/context"
	"free5gc/src/smpcf/logger"
	"free5gc/src/smpcf/util"
	"net/http"
)

func SendSearchNFInstances(nrfUri string, targetNfType, requestNfType models.NfType,
	param Nnrf_NFDiscovery.SearchNFInstancesParamOpts) (result models.SearchResult, err error) {
	configuration := Nnrf_NFDiscovery.NewConfiguration()
	configuration.SetBasePath(nrfUri)
	client := Nnrf_NFDiscovery.NewAPIClient(configuration)

	var res *http.Response
	result, res, err = client.NFInstancesStoreApi.SearchNFInstances(context.TODO(), targetNfType, requestNfType, &param)
	if res != nil && res.StatusCode == http.StatusTemporaryRedirect {
		err = fmt.Errorf("Temporary Redirect For Non NRF Consumer")
	}
	return
}

//20210619 added from  /src/pcf/nf_discovery.go
func SearchAvailableAMFs(nrfUri string, serviceName models.ServiceName) (
	amfInfos []pcf_context.AMFStatusSubscriptionData) {
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	result, err := SendSearchNFInstances(nrfUri, models.NfType_AMF, models.NfType_SMPCF, localVarOptionals)
	if err != nil {
		logger.PCFConsumerlog.Error(err.Error())
		return
	}

	for _, profile := range result.NfInstances {
		uri := util.SearchNFServiceUri(profile, serviceName, models.NfServiceStatus_REGISTERED)
		if uri != "" {
			item := pcf_context.AMFStatusSubscriptionData{
				AmfUri:    uri,
				GuamiList: *profile.AmfInfo.GuamiList,
			}
			amfInfos = append(amfInfos, item)
		}
	}
	return
}

//20210620 added from  /src/pcf/nf_discovery.go
func SendNFIntancesUDR(nrfUri, id string) string {
	targetNfType := models.NfType_UDR
	requestNfType := models.NfType_SMPCF
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		// 	DataSet: optional.NewInterface(models.DataSetId_SUBSCRIPTION),
	}
	// switch types {
	// case NFDiscoveryToUDRParamSupi:
	// 	localVarOptionals.Supi = optional.NewString(id)
	// case NFDiscoveryToUDRParamExtGroupId:
	// 	localVarOptionals.ExternalGroupIdentity = optional.NewString(id)
	// case NFDiscoveryToUDRParamGpsi:
	// 	localVarOptionals.Gpsi = optional.NewString(id)
	// }

	result, err := SendSearchNFInstances(nrfUri, targetNfType, requestNfType, localVarOptionals)
	if err != nil {
		logger.PCFConsumerlog.Error(err.Error())
		return ""
	}
	for _, profile := range result.NfInstances {
		if uri := util.SearchNFServiceUri(profile, models.ServiceName_NUDR_DR, models.NfServiceStatus_REGISTERED); uri != "" {
			return uri
		}
	}
	return ""
}
