package consumer

import (
	"context"
	"fmt"
	"free5gc/lib/openapi"
	"free5gc/lib/openapi/Nnrf_NFDiscovery"
	"free5gc/lib/openapi/Nudm_SubscriberDataManagement"
	"free5gc/lib/openapi/models"
	smaf_context "free5gc/src/smaf/context"
	"free5gc/src/smaf/logger"
	"net/http"

	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/mohae/deepcopy"
)

func SendNFRegistration() error {

	//set nfProfile
	profile := models.NfProfile{
		NfInstanceId:  smaf_context.SMAF_Self().NfInstanceID,
		NfType:        models.NfType_SMAF,
		NfStatus:      models.NfStatus_REGISTERED,
		Ipv4Addresses: []string{smaf_context.SMAF_Self().RegisterIPv4},
		NfServices:    smaf_context.NFServices,
		SmafInfo:      smaf_context.SmafInfo,
	}
	var rep models.NfProfile
	var res *http.Response
	var err error

	// Check data (Use RESTful PUT)
	for {
		rep, res, err = smaf_context.SMAF_Self().
			NFManagementClient.
			NFInstanceIDDocumentApi.
			RegisterNFInstance(context.TODO(), smaf_context.SMAF_Self().NfInstanceID, profile)
		if err != nil || res == nil {
			logger.AppLog.Infof("SMAF register to NRF Error[%s]", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}

		status := res.StatusCode
		if status == http.StatusOK {
			// NFUpdate
			break
		} else if status == http.StatusCreated {
			// NFRegister
			resourceUri := res.Header.Get("Location")
			// resouceNrfUri := resourceUri[strings.LastIndex(resourceUri, "/"):]
			smaf_context.SMAF_Self().NfInstanceID = resourceUri[strings.LastIndex(resourceUri, "/")+1:]
			break
		} else {
			logger.AppLog.Infof("handler returned wrong status code %d", status)
			// fmt.Errorf("NRF return wrong status code %d", status)
		}
	}

	logger.InitLog.Infof("SMAF Registration to NRF %v", rep)
	return nil
}

func RetrySendNFRegistration(MaxRetry int) error {

	retryCount := 0
	for retryCount < MaxRetry {
		err := SendNFRegistration()
		if err == nil {
			return nil
		}
		logger.AppLog.Warnf("Send NFRegistration Failed by %v", err)
		retryCount++
	}

	return fmt.Errorf("[SMAF] Retry NF Registration has meet maximum")
}

func SendNFDeregistration() error {

	// Check data (Use RESTful DELETE)
	res, localErr := smaf_context.SMAF_Self().
		NFManagementClient.
		NFInstanceIDDocumentApi.
		DeregisterNFInstance(context.TODO(), smaf_context.SMAF_Self().NfInstanceID)
	if localErr != nil {
		logger.AppLog.Warnln(localErr)
		return localErr
	}
	if res != nil {
		if status := res.StatusCode; status != http.StatusNoContent {
			logger.AppLog.Warnln("handler returned wrong status code ", status)
			return openapi.ReportError("handler returned wrong status code %d", status)
		}
	}
	return nil
}

func SendNFDiscoveryUDM() (*models.ProblemDetails, error) {

	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	// Check data
	result, httpResp, localErr := smaf_context.SMAF_Self().
		NFDiscoveryClient.
		NFInstancesStoreApi.
		SearchNFInstances(context.TODO(), models.NfType_UDM, models.NfType_SMAF, &localVarOptionals)

	if localErr == nil {
		smaf_context.SMAF_Self().UDMProfile = result.NfInstances[0]

		for _, service := range *smaf_context.SMAF_Self().UDMProfile.NfServices {
			if service.ServiceName == models.ServiceName_NUDM_SDM {
				SDMConf := Nudm_SubscriberDataManagement.NewConfiguration()
				SDMConf.SetBasePath(service.ApiPrefix)
				smaf_context.SMAF_Self().SubscriberDataManagementClient = Nudm_SubscriberDataManagement.NewAPIClient(SDMConf)
			}
		}

		if smaf_context.SMAF_Self().SubscriberDataManagementClient == nil {
			logger.AppLog.Warnln("sdm client failed")
		}
	} else if httpResp != nil {
		logger.AppLog.Warnln("handler returned wrong status code ", httpResp.Status)
		if httpResp.Status != localErr.Error() {
			return nil, localErr
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		return &problem, nil
	} else {
		return nil, openapi.ReportError("server no response")
	}
	return nil, nil
}

func SendNFDiscoveryPCF() (problemDetails *models.ProblemDetails, err error) {

	// Set targetNfType
	targetNfType := models.NfType_PCF
	// Set requestNfType
	requesterNfType := models.NfType_SMF
	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	// Check data
	result, httpResp, localErr := smaf_context.SMAF_Self().
		NFDiscoveryClient.
		NFInstancesStoreApi.
		SearchNFInstances(context.TODO(), targetNfType, requesterNfType, &localVarOptionals)

	if localErr == nil {
		logger.AppLog.Traceln(result.NfInstances)
	} else if httpResp != nil {
		logger.AppLog.Warnln("handler returned wrong status code ", httpResp.Status)
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}

	return
}

func SendNFDiscoveryServingAMF(smContext *smaf_context.SMContext) (*models.ProblemDetails, error) {
	targetNfType := models.NfType_AMF
	requesterNfType := models.NfType_SMAF

	localVarOptionals := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{}

	localVarOptionals.TargetNfInstanceId = optional.NewInterface(smContext.ServingNfId)

	// Check data
	result, httpResp, localErr := smaf_context.SMAF_Self().
		NFDiscoveryClient.
		NFInstancesStoreApi.
		SearchNFInstances(context.TODO(), targetNfType, requesterNfType, &localVarOptionals)

	if localErr == nil {
		if result.NfInstances == nil {
			if status := httpResp.StatusCode; status != http.StatusOK {
				logger.AppLog.Warnln("handler returned wrong status code", status)
			}
			logger.AppLog.Warnln("NfInstances is nil")
			return nil, openapi.ReportError("NfInstances is nil")
		}
		logger.AppLog.Info("SendNFDiscoveryServingAMF ok")
		smContext.AMFProfile = deepcopy.Copy(result.NfInstances[0]).(models.NfProfile)
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			return nil, localErr
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		return &problem, nil
	} else {
		return nil, openapi.ReportError("server no response")
	}

	return nil, nil

}

func SendDeregisterNFInstance() (*models.ProblemDetails, error) {
	logger.AppLog.Infof("Send Deregister NFInstance")

	smfSelf := smaf_context.SMAF_Self()
	// Set client and set url

	var res *http.Response

	var err error
	res, err = smfSelf.
		NFManagementClient.
		NFInstanceIDDocumentApi.
		DeregisterNFInstance(context.Background(), smfSelf.NfInstanceID)
	if err == nil {
		return nil, err
	} else if res != nil {
		if res.Status != err.Error() {
			return nil, err
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		return &problem, err
	} else {
		return nil, openapi.ReportError("server no response")
	}
}
