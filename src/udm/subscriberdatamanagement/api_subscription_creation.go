/*
 * Nudm_SDM
 *
 * Nudm Subscriber Data Management Service
 *
 * API version: 2.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package subscriberdatamanagement

import (
	"free5gc/lib/http_wrapper"
	"free5gc/lib/openapi"
	"free5gc/lib/openapi/models"
	"free5gc/src/udm/logger"
	"free5gc/src/udm/producer"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Subscribe - subscribe to notifications
func HTTPSubscribe(c *gin.Context) {

	var sdmSubscriptionReq models.SdmSubscription

	// step 1: retrieve http request body
	requestBody, err := c.GetRawData()
	if err != nil {
		problemDetail := models.ProblemDetails{
			Title:  "System failure",
			Status: http.StatusInternalServerError,
			Detail: err.Error(),
			Cause:  "SYSTEM_FAILURE",
		}
		logger.SdmLog.Errorf("Get Request Body error: %+v", err)
		c.JSON(http.StatusInternalServerError, problemDetail)
		return
	}

	// step 2: convert requestBody to openapi models
	err = openapi.Deserialize(&sdmSubscriptionReq, requestBody, "application/json")
	if err != nil {
		problemDetail := "[Request Body] " + err.Error()
		rsp := models.ProblemDetails{
			Title:  "Malformed request syntax",
			Status: http.StatusBadRequest,
			Detail: problemDetail,
		}
		logger.SdmLog.Errorln(problemDetail)
		c.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := http_wrapper.NewRequest(c.Request, sdmSubscriptionReq)
	req.Params["supi"] = c.Params.ByName("supi")

	rsp := producer.HandleSubscribeRequest(req)

	for key, val := range rsp.Header { // header response is optional
		c.Header(key, val[0])
	}
	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
	if err != nil {
		logger.SdmLog.Errorln(err)
		problemDetails := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, problemDetails)
	} else {
		c.Data(rsp.Status, "application/json", responseBody)
	}

}