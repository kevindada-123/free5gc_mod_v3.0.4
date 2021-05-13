/*
 * NRF NFManagement Service
 *
 * NRF NFManagement Service
 *
 * API version: 1.0.1
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package models

type SmafInfo struct {
	SNssaiSmafInfoList *[]SnssaiSmafInfoItem `json:"sNssaiSmafInfoList" yaml:"sNssaiSmafInfoList" bson:"sNssaiSmafInfoList" mapstructure:"SNssaiSmafInfoList"`
	TaiList            *[]Tai                `json:"taiList,omitempty" yaml:"taiList" bson:"taiList" mapstructure:"TaiList"`
	TaiRangeList       *[]TaiRange           `json:"taiRangeList,omitempty" yaml:"taiRangeList" bson:"taiRangeList" mapstructure:"TaiRangeList"`
	PgwFqdn            string                `json:"pgwFqdn,omitempty" yaml:"pgwFqdn" bson:"pgwFqdn" mapstructure:"PgwFqdn"`
	AccessType         []AccessType          `json:"accessType,omitempty" yaml:"accessType" bson:"accessType" mapstructure:"AccessType"`
}