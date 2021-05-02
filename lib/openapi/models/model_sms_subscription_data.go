/*
 * Nudm_SDM
 *
 * Nudm Subscriber Data Management Service
 *
 * API version: 2.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package models

type SmsSubscriptionData struct {
	SmsSubscribed       bool     `json:"smsSubscribed,omitempty" yaml:"smsSubscribed" bson:"smsSubscribed" mapstructure:"SmsSubscribed"`
	SharedSmsSubsDataId []string `json:"sharedSmsSubsDataId,omitempty" yaml:"sharedSmsSubsDataId" bson:"sharedSmsSubsDataId" mapstructure:"SharedSmsSubsDataId"`
}
