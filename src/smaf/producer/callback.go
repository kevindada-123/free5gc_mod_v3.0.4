package producer

import (
	"free5gc/lib/http_wrapper"
	"free5gc/lib/openapi/models"
	smaf_context "free5gc/src/smaf/context"
	"free5gc/src/smaf/logger"
	"net/http"
)

func HandleSMPolicyUpdateNotify(smContextRef string, request models.SmPolicyNotification) *http_wrapper.Response {
	logger.PduSessLog.Infoln("In HandleSMPolicyUpdateNotify")
	decision := request.SmPolicyDecision
	smContext := smaf_context.GetSMContext(smContextRef)
	if smContext == nil {
		logger.PduSessLog.Errorf("SMContext[%s] not found", smContextRef)
		httpResponse := http_wrapper.NewResponse(http.StatusBadRequest, nil, nil)
		return httpResponse
	}

	if smContext.SMContextState != smaf_context.Active {
		//Wait till the state becomes Active again
		//TODO: implement waiting in concurrent architecture
		logger.PduSessLog.Infoln("The SMContext State should be Active State")
		logger.PduSessLog.Infoln("SMContext state: ", smContext.SMContextState.String())
	}

	//TODO: Response data type -
	//[200 OK] UeCampingRep
	//[200 OK] array(PartialSuccessReport)
	//[400 Bad Request] ErrorReport
	httpResponse := http_wrapper.NewResponse(http.StatusNoContent, nil, nil)
	if err := ApplySmPolicyFromDecision(smContext, decision); err != nil {
		//TODO: Fill the error body
		httpResponse.Status = http.StatusBadRequest
	}

	return httpResponse
}

func handleSessionRule(smContext *smaf_context.SMContext, id string, sessionRuleModel *models.SessionRule) {
	if sessionRuleModel == nil {
		logger.PduSessLog.Debugf("Delete SessionRule[%s]", id)
		delete(smContext.SessionRules, id)
	} else {
		sessRule := smaf_context.NewSessionRuleFromModel(sessionRuleModel)
		// Session rule installation
		if oldSessRule, exist := smContext.SessionRules[id]; !exist {
			logger.PduSessLog.Debugf("Install SessionRule[%s]", id)
			smContext.SessionRules[id] = sessRule
		} else { // Session rule modification
			logger.PduSessLog.Debugf("Modify SessionRule[%s]", oldSessRule.SessionRuleID)
			smContext.SessionRules[id] = sessRule
		}
	}
}

func ApplySmPolicyFromDecision(smContext *smaf_context.SMContext, decision *models.SmPolicyDecision) error {
	logger.PduSessLog.Traceln("In ApplySmPolicyFromDecision")
	smContext.SMContextState = smaf_context.ModificationPending
	selectedSessionRule := smContext.SelectedSessionRule()
	if selectedSessionRule == nil { //No active session rule
		//Update session rules from decision
		for id, sessRuleModel := range decision.SessRules {
			handleSessionRule(smContext, id, &sessRuleModel)
		}
		for id := range smContext.SessionRules {
			// Randomly choose a session rule to activate
			smaf_context.SetSessionRuleActivateState(smContext.SessionRules[id], true)
			break
		}
	} else {
		selectedSessionRuleID := selectedSessionRule.SessionRuleID
		//Update session rules from decision
		for id, sessRuleModel := range decision.SessRules {
			handleSessionRule(smContext, id, &sessRuleModel)
		}
		if _, exist := smContext.SessionRules[selectedSessionRuleID]; !exist {
			//Original active session rule is deleted; choose again
			for id := range smContext.SessionRules {
				// Randomly choose a session rule to activate
				smaf_context.SetSessionRuleActivateState(smContext.SessionRules[id], true)
				break
			}
		} else {
			//Activate original active session rule
			smaf_context.SetSessionRuleActivateState(smContext.SessionRules[selectedSessionRuleID], true)
		}
	}
	logger.PduSessLog.Traceln("End of ApplySmPolicyFromDecision")
	return nil
}
