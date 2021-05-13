package consumer

import (
	"context"
	"free5gc/lib/openapi"
	"free5gc/lib/openapi/Nsmaf_PDUSession"
	"free5gc/lib/openapi/models"
	"free5gc/src/smaf/logger"
	"net/http"
)

//Example Consumer(s):AMF
func SendSMContextStatusNotification(uri string) (*models.ProblemDetails, error) {
	if uri != "" {
		request := models.SmContextStatusNotification{}
		request.StatusInfo = &models.StatusInfo{
			ResourceStatus: models.ResourceStatus_RELEASED,
		}
		configuration := Nsmaf_PDUSession.NewConfiguration()
		client := Nsmaf_PDUSession.NewAPIClient(configuration)

		//TS23.502 5.2.8.2.8	Nsmf_PDUSession_SMContextStatusNotify service operation
		logger.CtxLog.Infoln("[SMAF] Send SMContext Status Notification")
		httpResp, localErr := client.
			IndividualSMContextNotificationApi.
			SMContextNotification(context.Background(), uri, request)

		if localErr == nil {
			if httpResp.StatusCode != http.StatusNoContent {
				return nil, openapi.ReportError("Send SMContextStatus Notification Failed")

			}

			logger.PduSessLog.Tracef("Send SMContextStatus Notification Success")
		} else if httpResp != nil {
			logger.PduSessLog.Warnf("Send SMContextStatus Notification Error[%s]", httpResp.Status)
			if httpResp.Status != localErr.Error() {
				return nil, localErr
			}
			problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
			return &problem, nil
		} else {
			logger.PduSessLog.Warnln("Http Response is nil in comsumer API SMContextNotification")
			return nil, openapi.ReportError("Send SMContextStatus Notification Failed[%s]", localErr.Error())
		}

	}
	return nil, nil
}
