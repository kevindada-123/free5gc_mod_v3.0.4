/*
 * Nsmf_PDUSession
 *
 * SMF PDU Session Service
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package models

type NonDynamic5Qi struct {
	PriorityLevel   int32 `json:"priorityLevel,omitempty"`
	AverWindow      int32 `json:"averWindow,omitempty"`
	MaxDataBurstVol int32 `json:"maxDataBurstVol,omitempty"`
}