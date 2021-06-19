package consumer

import (
	"context"
	"fmt"
	"free5gc/lib/openapi/Nnrf_NFDiscovery"
	"free5gc/lib/openapi/models"
	pcf_context "free5gc/src/smaf/context"
	"free5gc/src/smaf/logger"
	"free5gc/src/smaf/util"
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

	result, err := SendSearchNFInstances(nrfUri, models.NfType_AMF, models.NfType_SMAF, localVarOptionals)
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
