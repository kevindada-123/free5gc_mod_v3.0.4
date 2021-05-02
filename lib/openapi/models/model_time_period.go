/*
 * Nudr_DataRepository API OpenAPI file
 *
 * Unified Data Repository Service
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package models

// Contains the periodicity for the defined usage monitoring data limits.
type TimePeriod struct {
	Period       Periodicity `json:"period" bson:"period"`
	MaxNumPeriod int32       `json:"maxNumPeriod,omitempty" bson:"maxNumPeriod"`
}
