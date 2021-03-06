package producer

import (
	"context"
	"fmt"
	"free5gc/lib/http_wrapper"
	"free5gc/lib/nas"
	"free5gc/lib/nas/nasConvert"
	"free5gc/lib/nas/nasMessage"
	"free5gc/lib/openapi"
	"free5gc/lib/openapi/Namf_Communication"
	"free5gc/lib/openapi/Nsmf_PDUSession"
	"free5gc/lib/openapi/Nudm_SubscriberDataManagement"
	"free5gc/lib/openapi/models"
	"free5gc/lib/pfcp/pfcpType"
	"free5gc/src/smaf/consumer"
	smaf_context "free5gc/src/smaf/context"
	"free5gc/src/smaf/logger"
	pfcp_message "free5gc/src/smaf/pfcp/message"
	"net/http"

	"github.com/antihax/optional"
)

func HandlePDUSessionSMContextCreate(request models.PostSmContextsRequest) *http_wrapper.Response {
	//GSM State
	//PDU Session Establishment Accept/Reject
	var response models.PostSmContextsResponse
	response.JsonData = new(models.SmContextCreatedData)
	logger.PduSessLog.Infoln("In HandlePDUSessionSMContextCreate")

	// Check has PDU Session Establishment Request
	m := nas.NewMessage()
	if err := m.GsmMessageDecode(&request.BinaryDataN1SmMessage); err != nil ||
		m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		logger.PduSessLog.Warnln("GsmMessageDecode Error: ", err)
		httpResponse := &http_wrapper.Response{
			Header: nil,
			Status: http.StatusForbidden,
			Body: models.PostSmContextsErrorResponse{
				JsonData: &models.SmContextCreateError{
					Error: &Nsmf_PDUSession.N1SmError,
				},
			},
		}
		return httpResponse
	}

	createData := request.JsonData
	smContext := smaf_context.NewSMContext(createData.Supi, createData.PduSessionId)
	smContext.SMContextState = smaf_context.ActivePending
	logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
	smContext.SetCreateData(createData)
	smContext.SmStatusNotifyUri = createData.SmContextStatusUri

	// Query UDM
	if problemDetails, err := consumer.SendNFDiscoveryUDM(); err != nil {
		logger.PduSessLog.Warnf("Send NF Discovery Serving UDM Error[%v]", err)
	} else if problemDetails != nil {
		logger.PduSessLog.Warnf("Send NF Discovery Serving UDM Problem[%+v]", problemDetails)
	} else {
		logger.PduSessLog.Infoln("Send NF Discovery Serving UDM Successfully")
	}

	smPlmnID := createData.Guami.PlmnId

	smDataParams := &Nudm_SubscriberDataManagement.GetSmDataParamOpts{
		Dnn:         optional.NewString(createData.Dnn),
		PlmnId:      optional.NewInterface(smPlmnID.Mcc + smPlmnID.Mnc),
		SingleNssai: optional.NewInterface(openapi.MarshToJsonString(smContext.Snssai)),
	}

	SubscriberDataManagementClient := smaf_context.SMAF_Self().SubscriberDataManagementClient

	if sessSubData, _, err := SubscriberDataManagementClient.
		SessionManagementSubscriptionDataRetrievalApi.
		GetSmData(context.Background(), smContext.Supi, smDataParams); err != nil {
		logger.PduSessLog.Errorln("Get SessionManagementSubscriptionData error:", err)
	} else {
		if len(sessSubData) > 0 {
			smContext.DnnConfiguration = sessSubData[0].DnnConfigurations[smContext.Dnn]
		} else {
			logger.PduSessLog.Errorln("SessionManagementSubscriptionData from UDM is nil")
		}
	}

	establishmentRequest := m.PDUSessionEstablishmentRequest
	smContext.HandlePDUSessionEstablishmentRequest(establishmentRequest)

	logger.PduSessLog.Infof("PCF Selection for SMContext SUPI[%s] PDUSessionID[%d]\n",
		smContext.Supi, smContext.PDUSessionID)
	if err := smContext.PCFSelection(); err != nil {
		logger.PduSessLog.Errorln("pcf selection error:", err)
	}

	smPolicyData := models.SmPolicyContextData{}

	smPolicyData.Supi = smContext.Supi
	smPolicyData.PduSessionId = smContext.PDUSessionID
	smPolicyData.NotificationUri = fmt.Sprintf("%s://%s:%d/nsmf-callback/sm-policies/%s",
		smaf_context.SMAF_Self().UriScheme,
		smaf_context.SMAF_Self().RegisterIPv4,
		smaf_context.SMAF_Self().SBIPort,
		smContext.Ref,
	)
	smPolicyData.Dnn = smContext.Dnn
	smPolicyData.PduSessionType = nasConvert.PDUSessionTypeToModels(smContext.SelectedPDUSessionType)
	smPolicyData.AccessType = smContext.AnType
	smPolicyData.RatType = smContext.RatType
	smPolicyData.Ipv4Address = smContext.PDUAddress.To4().String()
	smPolicyData.SubsSessAmbr = smContext.DnnConfiguration.SessionAmbr
	smPolicyData.SubsDefQos = smContext.DnnConfiguration.Var5gQosProfile
	smPolicyData.SliceInfo = smContext.Snssai
	smPolicyData.ServingNetwork = &models.NetworkId{
		Mcc: smContext.ServingNetwork.Mcc,
		Mnc: smContext.ServingNetwork.Mnc,
	}
	smPolicyData.SuppFeat = "F"

	var smPolicyDecision models.SmPolicyDecision
	/*
		if smPolicyDecisionFromPCF, _, err := smContext.SMPolicyClient.
			DefaultApi.SmPoliciesPost(context.Background(), smPolicyData); err != nil {
			openapiError := err.(openapi.GenericOpenAPIError)
			problemDetails := openapiError.Model().(models.ProblemDetails)
			logger.PduSessLog.Errorln("setup sm policy association failed:", err, problemDetails)
		} else {
			smPolicyDecision = smPolicyDecisionFromPCF
		}
	*/
	smPolicyDecisionFromPCF, _, err := smContext.SMPolicyClient.DefaultApi.SmPoliciesPost(context.Background(), smPolicyData)
	if err != nil {
		openapiError := err.(openapi.GenericOpenAPIError)
		problemDetails := openapiError.Model().(models.ProblemDetails)
		logger.PduSessLog.Errorln("setup sm policy association failed:", err, problemDetails)
	} else {
		smPolicyDecision = smPolicyDecisionFromPCF
	}
	if err := ApplySmPolicyFromDecision(smContext, &smPolicyDecision); err != nil {
		logger.PduSessLog.Errorf("apply sm policy decision error: %+v", err)
	}

	smContext.Tunnel = smaf_context.NewUPTunnel()
	var defaultPath *smaf_context.DataPath

	if smaf_context.SMAF_Self().ULCLSupport && smaf_context.CheckUEHasPreConfig(createData.Supi) {
		logger.PduSessLog.Infof("SUPI[%s] has pre-config route", createData.Supi)
		uePreConfigPaths := smaf_context.GetUEPreConfigPaths(createData.Supi)
		smContext.Tunnel.DataPathPool = uePreConfigPaths.DataPathPool
		smContext.Tunnel.PathIDGenerator = uePreConfigPaths.PathIDGenerator
		defaultPath = smContext.Tunnel.DataPathPool.GetDefaultPath()
		smContext.AllocateLocalSEIDForDataPath(defaultPath)
		defaultPath.ActivateTunnelAndPDR(smContext)
		smContext.BPManager = smaf_context.NewBPManager(createData.Supi)
	} else {
		//UE has no pre-config path.
		//Use default route
		logger.PduSessLog.Infof("SUPI[%s] has no pre-config route", createData.Supi)
		//fmt.Printf("createData : %#v \n", createData)
		defaultUPPath := smaf_context.GetUserPlaneInformation().GetDefaultUserPlanePathByDNN(createData.Dnn)
		smContext.AllocateLocalSEIDForUPPath(defaultUPPath)
		defaultPath = smaf_context.GenerateDataPath(defaultUPPath, smContext)
		if defaultPath != nil {
			defaultPath.IsDefaultPath = true
			smContext.Tunnel.AddDataPath(defaultPath)
			defaultPath.ActivateTunnelAndPDR(smContext)
		}
	}

	if defaultPath == nil {
		smContext.SMContextState = smaf_context.InActive
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		logger.PduSessLog.Warnf("Path for serve DNN[%s] not found\n", createData.Dnn)

		var httpResponse *http_wrapper.Response
		if buf, err := smaf_context.
			BuildGSMPDUSessionEstablishmentReject(
				smContext,
				nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN); err != nil {
			httpResponse = &http_wrapper.Response{
				Header: nil,
				Status: http.StatusForbidden,
				Body: models.PostSmContextsErrorResponse{
					JsonData: &models.SmContextCreateError{
						Error:   &Nsmf_PDUSession.InsufficientResourceSliceDnn,
						N1SmMsg: &models.RefToBinaryData{ContentId: "n1SmMsg"},
					},
				},
			}
		} else {
			httpResponse = &http_wrapper.Response{
				Header: nil,
				Status: http.StatusForbidden,
				Body: models.PostSmContextsErrorResponse{
					JsonData: &models.SmContextCreateError{
						Error:   &Nsmf_PDUSession.InsufficientResourceSliceDnn,
						N1SmMsg: &models.RefToBinaryData{ContentId: "n1SmMsg"},
					},
					BinaryDataN1SmMessage: buf,
				},
			}
		}

		return httpResponse

	}

	if problemDetails, err := consumer.SendNFDiscoveryServingAMF(smContext); err != nil {
		logger.PduSessLog.Warnf("Send NF Discovery Serving AMF Error[%v]", err)
	} else if problemDetails != nil {
		logger.PduSessLog.Warnf("Send NF Discovery Serving AMF Problem[%+v]", problemDetails)
	} else {
		logger.PduSessLog.Traceln("Send NF Discovery Serving AMF successfully")
	}

	for _, service := range *smContext.AMFProfile.NfServices {
		if service.ServiceName == models.ServiceName_NAMF_COMM {
			communicationConf := Namf_Communication.NewConfiguration()
			communicationConf.SetBasePath(service.ApiPrefix)
			smContext.CommunicationClient = Namf_Communication.NewAPIClient(communicationConf)
		}
	}
	SendPFCPRule(smContext, defaultPath)

	response.JsonData = smContext.BuildCreatedData()
	httpResponse := &http_wrapper.Response{
		Header: http.Header{
			"Location": {smContext.Ref},
		},
		Status: http.StatusCreated,
		Body:   response,
	}

	return httpResponse
	// TODO: UECM registration

}

func HandlePDUSessionSMContextUpdate(smContextRef string, body models.UpdateSmContextRequest) *http_wrapper.Response {
	//GSM State
	//PDU Session Modification Reject(Cause Value == 43 || Cause Value != 43)/Complete
	//PDU Session Release Command/Complete
	logger.PduSessLog.Infoln("In HandlePDUSessionSMContextUpdate")
	smContext := smaf_context.GetSMContext(smContextRef)

	if smContext == nil {
		logger.PduSessLog.Warnln("SMContext is nil")

		httpResponse := &http_wrapper.Response{
			Header: nil,
			Status: http.StatusNotFound,
			Body: models.UpdateSmContextErrorResponse{
				JsonData: &models.SmContextUpdateError{
					UpCnxState: models.UpCnxState_DEACTIVATED,
					Error: &models.ProblemDetails{
						Type:   "Resource Not Found",
						Title:  "SMContext Ref is not found",
						Status: http.StatusNotFound,
					},
				},
			},
		}
		return httpResponse
	}

	var sendPFCPDelete, sendPFCPModification bool
	var response models.UpdateSmContextResponse
	response.JsonData = new(models.SmContextUpdatedData)

	smContextUpdateData := body.JsonData

	if body.BinaryDataN1SmMessage != nil {
		logger.PduSessLog.Traceln("Binary Data N1 SmMessage isn't nil!")
		m := nas.NewMessage()
		err := m.GsmMessageDecode(&body.BinaryDataN1SmMessage)
		logger.PduSessLog.Traceln("[SMAF] UpdateSmContextRequest N1SmMessage: ", m)
		if err != nil {
			logger.PduSessLog.Error(err)
			httpResponse := &http_wrapper.Response{
				Status: http.StatusForbidden,
				Body: models.UpdateSmContextErrorResponse{
					JsonData: &models.SmContextUpdateError{
						Error: &Nsmf_PDUSession.N1SmError,
					},
				}, //Depends on the reason why N4 fail
			}
			return httpResponse
		}
		switch m.GsmHeader.GetMessageType() {
		case nas.MsgTypePDUSessionReleaseRequest:
			if smContext.SMContextState != smaf_context.Active {
				//Wait till the state becomes Active again
				//TODO: implement sleep wait in concurrent architecture
				logger.PduSessLog.Infoln("The SMContext State should be Active State")
				logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
			}

			smContext.HandlePDUSessionReleaseRequest(m.PDUSessionReleaseRequest)
			if buf, err := smaf_context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
				logger.PduSessLog.Errorf("Build GSM PDUSessionReleaseCommand failed: %+v", err)
			} else {
				response.BinaryDataN1SmMessage = buf
			}

			response.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseCommand"}

			response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "PDUResourceReleaseCommand"}
			response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_REL_CMD

			if buf, err := smaf_context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
				logger.PduSessLog.Errorf("Build PDUSessionResourceReleaseCommandTransfer failed: %+v", err)
			} else {
				response.BinaryDataN2SmInformation = buf
			}

			deletedPFCPNode := make(map[string]bool)
			smContext.PendingUPF = make(smaf_context.PendingUPF)
			for _, dataPath := range smContext.Tunnel.DataPathPool {

				dataPath.DeactivateTunnelAndPDR(smContext)
				for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
					curUPFID, err := curDataPathNode.GetUPFID()
					if err != nil {
						logger.PduSessLog.Error("DataPath UPFID not found")
						continue
					}
					if _, exist := deletedPFCPNode[curUPFID]; !exist {
						pfcp_message.SendPfcpSessionDeletionRequest(curDataPathNode.UPF.NodeID, smContext)
						deletedPFCPNode[curUPFID] = true
						smContext.PendingUPF[curDataPathNode.GetNodeIP()] = true
					}
				}
			}

			sendPFCPDelete = true
			smContext.SMContextState = smaf_context.PFCPModification
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		case nas.MsgTypePDUSessionReleaseComplete:
			if smContext.SMContextState != smaf_context.InActivePending {
				//Wait till the state becomes Active again
				//TODO: implement sleep wait in concurrent architecture
				logger.PduSessLog.Infoln("The SMContext State should be InActivePending State")
				logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
			}
			// Send Release Notify to AMF
			logger.PduSessLog.Infoln("[SMAF] Send Update SmContext Response")
			smContext.SMContextState = smaf_context.InActive
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			response.JsonData.UpCnxState = models.UpCnxState_DEACTIVATED
			smaf_context.RemoveSMContext(smContext.Ref)
			problemDetails, err := consumer.SendSMContextStatusNotification(smContext.SmStatusNotifyUri)
			if problemDetails != nil || err != nil {
				if problemDetails != nil {
					logger.PduSessLog.Warnf("Send SMContext Status Notification Problem[%+v]", problemDetails)
				}

				if err != nil {
					logger.PduSessLog.Warnf("Send SMContext Status Notification Error[%v]", err)
				}
			} else {
				logger.PduSessLog.Traceln("Send SMContext Status Notification successfully")
			}
		}

	} else {
		logger.PduSessLog.Traceln("[SMAF] Binary Data N1 SmMessage is nil!")
	}

	tunnel := smContext.Tunnel
	pdrList := []*smaf_context.PDR{}
	farList := []*smaf_context.FAR{}
	barList := []*smaf_context.BAR{}

	switch smContextUpdateData.UpCnxState {
	case models.UpCnxState_ACTIVATING:
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "PDUSessionResourceSetupRequestTransfer"}
		response.JsonData.UpCnxState = models.UpCnxState_ACTIVATING
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ

		n2Buf, err := smaf_context.BuildPDUSessionResourceSetupRequestTransfer(smContext)
		if err != nil {
			logger.PduSessLog.Errorf("Build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		}
		response.BinaryDataN2SmInformation = n2Buf
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ
	case models.UpCnxState_DEACTIVATED:
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		response.JsonData.UpCnxState = models.UpCnxState_DEACTIVATED
		smContext.UpCnxState = body.JsonData.UpCnxState
		smContext.UeLocation = body.JsonData.UeLocation
		// TODO: Deactivate N2 downlink tunnel
		//Set FAR and An, N3 Release Info
		farList = []*smaf_context.FAR{}
		smContext.PendingUPF = make(smaf_context.PendingUPF)
		for _, dataPath := range smContext.Tunnel.DataPathPool {

			ANUPF := dataPath.FirstDPNode
			DLPDR := ANUPF.DownLinkTunnel.PDR
			if DLPDR == nil {
				logger.PduSessLog.Errorf("AN Release Error")
			} else {
				DLPDR.FAR.State = smaf_context.RULE_UPDATE
				DLPDR.FAR.ApplyAction.Forw = false
				DLPDR.FAR.ApplyAction.Buff = true
				DLPDR.FAR.ApplyAction.Nocp = true
				smContext.PendingUPF[ANUPF.GetNodeIP()] = true
			}

			farList = append(farList, DLPDR.FAR)
		}

		sendPFCPModification = true
		smContext.SMContextState = smaf_context.PFCPModification
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
	}

	switch smContextUpdateData.N2SmInfoType {
	case models.N2SmInfoType_PDU_RES_SETUP_RSP:
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		pdrList = []*smaf_context.PDR{}
		farList = []*smaf_context.FAR{}

		smContext.PendingUPF = make(smaf_context.PendingUPF)
		for _, dataPath := range tunnel.DataPathPool {

			if dataPath.Activated {
				ANUPF := dataPath.FirstDPNode
				DLPDR := ANUPF.DownLinkTunnel.PDR

				DLPDR.FAR.ApplyAction = pfcpType.ApplyAction{Buff: false, Drop: false, Dupl: false, Forw: true, Nocp: false}
				DLPDR.FAR.ForwardingParameters = &smaf_context.ForwardingParameters{
					DestinationInterface: pfcpType.DestinationInterface{
						InterfaceValue: pfcpType.DestinationInterfaceAccess,
					},
					NetworkInstance: []byte(smContext.Dnn),
				}

				DLPDR.State = smaf_context.RULE_UPDATE
				DLPDR.FAR.State = smaf_context.RULE_UPDATE

				pdrList = append(pdrList, DLPDR)
				farList = append(farList, DLPDR.FAR)

				if _, exist := smContext.PendingUPF[ANUPF.GetNodeIP()]; !exist {
					smContext.PendingUPF[ANUPF.GetNodeIP()] = true
				}
			}

		}

		if err := smaf_context.
			HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.PduSessLog.Errorf("Handle PDUSessionResourceSetupResponseTransfer failed: %+v", err)
		}
		sendPFCPModification = true
		smContext.SMContextState = smaf_context.PFCPModification
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
	case models.N2SmInfoType_PDU_RES_REL_RSP:
		logger.PduSessLog.Infoln("[SMAF] N2 PDUSession Release Complete ")
		if smContext.PDUSessionRelease_DUE_TO_DUP_PDU_ID {
			if smContext.SMContextState != smaf_context.InActivePending {
				//Wait till the state becomes Active again
				//TODO: implement sleep wait in concurrent architecture
				logger.PduSessLog.Infoln("The SMContext State should be InActivePending State")
				logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
			}
			smContext.SMContextState = smaf_context.InActive
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			logger.PduSessLog.Infoln("[SMAF] Send Update SmContext Response")
			response.JsonData.UpCnxState = models.UpCnxState_DEACTIVATED

			smContext.PDUSessionRelease_DUE_TO_DUP_PDU_ID = false
			smaf_context.RemoveSMContext(smContext.Ref)
			problemDetails, err := consumer.SendSMContextStatusNotification(smContext.SmStatusNotifyUri)
			if problemDetails != nil || err != nil {
				if problemDetails != nil {
					logger.PduSessLog.Warnf("Send SMContext Status Notification Problem[%+v]", problemDetails)
				}

				if err != nil {
					logger.PduSessLog.Warnf("Send SMContext Status Notification Error[%v]", err)
				}
			} else {
				logger.PduSessLog.Traceln("Send SMContext Status Notification successfully")
			}

		} else { // normal case
			if smContext.SMContextState != smaf_context.InActivePending {
				//Wait till the state becomes Active again
				//TODO: implement sleep wait in concurrent architecture
				logger.PduSessLog.Infoln("The SMContext State should be InActivePending State")
				logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
			}
			logger.PduSessLog.Infoln("[SMAF] Send Update SmContext Response")
			smContext.SMContextState = smaf_context.InActivePending
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())

		}
	case models.N2SmInfoType_PATH_SWITCH_REQ:
		logger.PduSessLog.Traceln("Handle Path Switch Request")
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())

		if err := smaf_context.HandlePathSwitchRequestTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.PduSessLog.Errorf("Handle PathSwitchRequestTransfer: %+v", err)
		}

		if n2Buf, err := smaf_context.BuildPathSwitchRequestAcknowledgeTransfer(smContext); err != nil {
			logger.PduSessLog.Errorf("Build Path Switch Transfer Error(%+v)", err)
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JsonData.N2SmInfoType = models.N2SmInfoType_PATH_SWITCH_REQ_ACK
		response.JsonData.N2SmInfo = &models.RefToBinaryData{
			ContentId: "PATH_SWITCH_REQ_ACK",
		}

		smContext.PendingUPF = make(smaf_context.PendingUPF)
		for _, dataPath := range tunnel.DataPathPool {

			if dataPath.Activated {
				ANUPF := dataPath.FirstDPNode
				DLPDR := ANUPF.DownLinkTunnel.PDR

				pdrList = append(pdrList, DLPDR)
				farList = append(farList, DLPDR.FAR)

				if _, exist := smContext.PendingUPF[ANUPF.GetNodeIP()]; !exist {
					smContext.PendingUPF[ANUPF.GetNodeIP()] = true
				}
			}
		}

		sendPFCPModification = true
		smContext.SMContextState = smaf_context.PFCPModification
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
	case models.N2SmInfoType_PATH_SWITCH_SETUP_FAIL:
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		if err :=
			smaf_context.HandlePathSwitchRequestSetupFailedTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.PduSessLog.Error()
		}
	case models.N2SmInfoType_HANDOVER_REQUIRED:
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "Handover"}
	}

	switch smContextUpdateData.HoState {
	case models.HoState_PREPARING:
		logger.PduSessLog.Traceln("In HoState_PREPARING")
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		smContext.HoState = models.HoState_PREPARING
		if err := smaf_context.HandleHandoverRequiredTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.PduSessLog.Errorf("Handle HandoverRequiredTransfer failed: %+v", err)
		}
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ

		if n2Buf, err := smaf_context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			logger.PduSessLog.Errorf("Build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ
		response.JsonData.N2SmInfo = &models.RefToBinaryData{
			ContentId: "PDU_RES_SETUP_REQ",
		}
		response.JsonData.HoState = models.HoState_PREPARING
	case models.HoState_PREPARED:
		logger.PduSessLog.Traceln("In HoState_PREPARED")
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		smContext.HoState = models.HoState_PREPARED
		response.JsonData.HoState = models.HoState_PREPARED
		if err :=
			smaf_context.HandleHandoverRequestAcknowledgeTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.PduSessLog.Errorf("Handle HandoverRequestAcknowledgeTransfer failed: %+v", err)
		}

		if n2Buf, err := smaf_context.BuildHandoverCommandTransfer(smContext); err != nil {
			logger.PduSessLog.Errorf("Build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JsonData.N2SmInfoType = models.N2SmInfoType_HANDOVER_CMD
		response.JsonData.N2SmInfo = &models.RefToBinaryData{
			ContentId: "HANDOVER_CMD",
		}
		response.JsonData.HoState = models.HoState_PREPARING
	case models.HoState_COMPLETED:
		logger.PduSessLog.Traceln("In HoState_COMPLETED")
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}
		smContext.SMContextState = smaf_context.ModificationPending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		smContext.HoState = models.HoState_COMPLETED
		response.JsonData.HoState = models.HoState_COMPLETED
	}

	switch smContextUpdateData.Cause {

	case models.Cause_REL_DUE_TO_DUPLICATE_SESSION_ID:
		//* release PDU Session Here
		if smContext.SMContextState != smaf_context.Active {
			//Wait till the state becomes Active again
			//TODO: implement sleep wait in concurrent architecture
			logger.PduSessLog.Infoln("The SMContext State should be Active State")
			logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
		}

		response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "PDUResourceReleaseCommand"}
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_REL_CMD
		smContext.PDUSessionRelease_DUE_TO_DUP_PDU_ID = true

		buf, err := smaf_context.BuildPDUSessionResourceReleaseCommandTransfer(smContext)
		response.BinaryDataN2SmInformation = buf
		if err != nil {
			logger.PduSessLog.Error(err)
		}

		deletedPFCPNode := make(map[string]bool)
		smContext.PendingUPF = make(smaf_context.PendingUPF)
		for _, dataPath := range smContext.Tunnel.DataPathPool {

			dataPath.DeactivateTunnelAndPDR(smContext)
			for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
				curUPFID, err := curDataPathNode.GetUPFID()
				if err != nil {
					logger.PduSessLog.Error("DataPath UPFID not found")
					continue
				}
				if _, exist := deletedPFCPNode[curUPFID]; !exist {
					pfcp_message.SendPfcpSessionDeletionRequest(curDataPathNode.UPF.NodeID, smContext)
					deletedPFCPNode[curUPFID] = true
					smContext.PendingUPF[curDataPathNode.GetNodeIP()] = true
				}
			}
		}

		sendPFCPDelete = true
		smContext.SMContextState = smaf_context.PFCPModification
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		logger.SMAFContextLog.Infoln("[SMAF] Cause_REL_DUE_TO_DUPLICATE_SESSION_ID")
	}

	var httpResponse *http_wrapper.Response
	//Check FSM and take corresponding action
	switch smContext.SMContextState {
	case smaf_context.PFCPModification:
		logger.SMAFContextLog.Traceln("In case PFCPModification")

		if sendPFCPModification {
			defaultPath := smContext.Tunnel.DataPathPool.GetDefaultPath()
			ANUPF := defaultPath.FirstDPNode
			pfcp_message.SendPfcpSessionModificationRequest(ANUPF.UPF.NodeID, smContext, pdrList, farList, barList)
		}

		if sendPFCPDelete {
			logger.PduSessLog.Infoln("Send PFCP Deletion from HandlePDUSessionSMContextUpdate")
		}

		PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan

		switch PFCPResponseStatus {
		case smaf_context.SessionUpdateSuccess:
			logger.SMAFContextLog.Traceln("In case SessionUpdateSuccess")
			smContext.SMContextState = smaf_context.Active
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			httpResponse = &http_wrapper.Response{
				Status: http.StatusOK,
				Body:   response,
			}
		case smaf_context.SessionUpdateFailed:
			logger.SMAFContextLog.Traceln("In case SessionUpdateFailed")
			smContext.SMContextState = smaf_context.Active
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			//It is just a template
			httpResponse = &http_wrapper.Response{
				Status: http.StatusForbidden,
				Body: models.UpdateSmContextErrorResponse{
					JsonData: &models.SmContextUpdateError{
						Error: &Nsmf_PDUSession.N1SmError,
					},
				}, //Depends on the reason why N4 fail
			}

		case smaf_context.SessionReleaseSuccess:
			logger.SMAFContextLog.Traceln("In case SessionReleaseSuccess")
			smContext.SMContextState = smaf_context.InActivePending
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			httpResponse = &http_wrapper.Response{
				Status: http.StatusOK,
				Body:   response,
			}

		case smaf_context.SessionReleaseFailed:
			// Update SmContext Request(N1 PDU Session Release Request)
			// Send PDU Session Release Reject
			logger.SMAFContextLog.Traceln("In case SessionReleaseFailed")
			problemDetail := models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Cause:  "SYSTEM_FAILULE",
			}
			httpResponse = &http_wrapper.Response{
				Status: int(problemDetail.Status),
			}
			smContext.SMContextState = smaf_context.Active
			logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			errResponse := models.UpdateSmContextErrorResponse{
				JsonData: &models.SmContextUpdateError{
					Error: &problemDetail,
				},
			}
			if buf, err := smaf_context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
				logger.PduSessLog.Errorf("build GSM PDUSessionReleaseReject failed: %+v", err)
			} else {
				errResponse.BinaryDataN1SmMessage = buf
			}

			errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
			httpResponse.Body = errResponse

		}
	case smaf_context.ModificationPending:
		logger.SMAFContextLog.Traceln("In case ModificationPending")
		smContext.SMContextState = smaf_context.Active
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		httpResponse = &http_wrapper.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	case smaf_context.InActive, smaf_context.InActivePending:
		logger.SMAFContextLog.Traceln("In case InActive, InActivePending")
		httpResponse = &http_wrapper.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	default:
		logger.PduSessLog.Warnf("SM Context State [%s] shouldn't be here\n", smContext.SMContextState)
		httpResponse = &http_wrapper.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	}

	return httpResponse
}

func HandlePDUSessionSMContextRelease(smContextRef string, body models.ReleaseSmContextRequest) *http_wrapper.Response {
	logger.PduSessLog.Infoln("In HandlePDUSessionSMContextRelease")
	smContext := smaf_context.GetSMContext(smContextRef)
	// smaf_context.RemoveSMContext(smContext.Ref)

	deletedPFCPNode := make(map[string]bool)
	smContext.PendingUPF = make(smaf_context.PendingUPF)
	for _, dataPath := range smContext.Tunnel.DataPathPool {

		dataPath.DeactivateTunnelAndPDR(smContext)
		for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
			curUPFID, err := curDataPathNode.GetUPFID()
			if err != nil {
				logger.PduSessLog.Error("DataPath UPFID not found")
				continue
			}
			if _, exist := deletedPFCPNode[curUPFID]; !exist {
				pfcp_message.SendPfcpSessionDeletionRequest(curDataPathNode.UPF.NodeID, smContext)
				deletedPFCPNode[curUPFID] = true
				smContext.PendingUPF[curDataPathNode.GetNodeIP()] = true
			}
		}
	}

	smContext.SMContextState = smaf_context.PFCPModification
	logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())

	var httpResponse *http_wrapper.Response
	PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan

	switch PFCPResponseStatus {
	case smaf_context.SessionReleaseSuccess:
		logger.SMAFContextLog.Traceln("In case SessionReleaseSuccess")
		smContext.SMContextState = smaf_context.InActivePending
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		httpResponse = &http_wrapper.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}

	case smaf_context.SessionReleaseFailed:
		// Update SmContext Request(N1 PDU Session Release Request)
		// Send PDU Session Release Reject
		logger.SMAFContextLog.Traceln("In case SessionReleaseFailed")
		problemDetail := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILULE",
		}
		httpResponse = &http_wrapper.Response{
			Status: int(problemDetail.Status),
		}
		smContext.SMContextState = smaf_context.Active
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		errResponse := models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error: &problemDetail,
			},
		}
		if buf, err := smaf_context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
			logger.PduSessLog.Errorf("Build GSM PDUSessionReleaseReject failed: %+v", err)
		} else {
			errResponse.BinaryDataN1SmMessage = buf
		}

		errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
		httpResponse.Body = errResponse
	default:
		logger.SMAFContextLog.Warnf("The state shouldn't be [%s]\n", PFCPResponseStatus)

		logger.SMAFContextLog.Traceln("In case Unkown")
		problemDetail := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILULE",
		}
		httpResponse = &http_wrapper.Response{
			Status: int(problemDetail.Status),
		}
		smContext.SMContextState = smaf_context.Active
		logger.SMAFContextLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		errResponse := models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error: &problemDetail,
			},
		}
		if buf, err := smaf_context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
			logger.PduSessLog.Errorf("Build GSM PDUSessionReleaseReject failed: %+v", err)
		} else {
			errResponse.BinaryDataN1SmMessage = buf
		}

		errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
		httpResponse.Body = errResponse

	}

	return httpResponse
}

func SendPFCPRule(smContext *smaf_context.SMContext, dataPath *smaf_context.DataPath) {

	logger.PduSessLog.Infoln("Send PFCP Rule")
	logger.PduSessLog.Infoln("DataPath: ", dataPath)
	for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		pdrList := make([]*smaf_context.PDR, 0, 2)
		farList := make([]*smaf_context.FAR, 0, 2)

		sessionContext, exist := smContext.PFCPContext[curDataPathNode.GetNodeIP()]
		if !exist || sessionContext.RemoteSEID == 0 {
			if curDataPathNode.UpLinkTunnel != nil && curDataPathNode.UpLinkTunnel.PDR != nil {
				pdrList = append(pdrList, curDataPathNode.UpLinkTunnel.PDR)
				farList = append(farList, curDataPathNode.UpLinkTunnel.PDR.FAR)
			}
			if curDataPathNode.DownLinkTunnel != nil && curDataPathNode.DownLinkTunnel.PDR != nil {
				pdrList = append(pdrList, curDataPathNode.DownLinkTunnel.PDR)
				farList = append(farList, curDataPathNode.DownLinkTunnel.PDR.FAR)
			}

			pfcp_message.SendPfcpSessionEstablishmentRequest(curDataPathNode.UPF.NodeID, smContext, pdrList, farList, nil)
		} else {
			if curDataPathNode.UpLinkTunnel != nil && curDataPathNode.UpLinkTunnel.PDR != nil {
				pdrList = append(pdrList, curDataPathNode.UpLinkTunnel.PDR)
				farList = append(farList, curDataPathNode.UpLinkTunnel.PDR.FAR)
			}
			if curDataPathNode.DownLinkTunnel != nil && curDataPathNode.DownLinkTunnel.PDR != nil {
				pdrList = append(pdrList, curDataPathNode.DownLinkTunnel.PDR)
				farList = append(farList, curDataPathNode.DownLinkTunnel.PDR.FAR)
			}

			pfcp_message.SendPfcpSessionModificationRequest(curDataPathNode.UPF.NodeID, smContext, pdrList, farList, nil)
		}

	}
}
