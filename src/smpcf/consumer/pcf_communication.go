package consumer

import (
	"context"
	"fmt"
	"free5gc/lib/openapi"
	"free5gc/lib/openapi/models"
	smpcf_context "free5gc/src/smpcf/context"
	"free5gc/src/smpcf/logger"
	"free5gc/src/smpcf/util"
	"strings"
)

func AmfStatusChangeSubscribe(amfInfo smpcf_context.AMFStatusSubscriptionData) (
	problemDetails *models.ProblemDetails, err error) {
	logger.PCFConsumerlog.Debugf("SMPCF Subscribe to AMF status[%+v]", amfInfo.AmfUri)
	pcfSelf := smpcf_context.SMPCF_Self()
	client := util.GetNamfClient(amfInfo.AmfUri)

	subscriptionData := models.SubscriptionData{
		AmfStatusUri: fmt.Sprintf("%s/npcf-callback/v1/amfstatus", pcfSelf.GetIPv4Uri()),
		GuamiList:    amfInfo.GuamiList,
	}
	res, httpResp, localErr :=
		client.SubscriptionsCollectionDocumentApi.AMFStatusChangeSubscribe(context.Background(), subscriptionData)
	if localErr == nil {
		locationHeader := httpResp.Header.Get("Location")
		logger.PCFConsumerlog.Debugf("location header: %+v", locationHeader)

		subscriptionId := locationHeader[strings.LastIndex(locationHeader, "/")+1:]
		amfStatusSubsData := smpcf_context.AMFStatusSubscriptionData{
			AmfUri:       amfInfo.AmfUri,
			AmfStatusUri: res.AmfStatusUri,
			GuamiList:    res.GuamiList,
		}
		pcfSelf.AMFStatusSubsData[subscriptionId] = amfStatusSubsData
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", amfInfo.AmfUri)
	}
	return problemDetails, err
}
