// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Core Services API
//
// API covering the Networking (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/overview.htm),
// Compute (https://docs.cloud.oracle.com/iaas/Content/Compute/Concepts/computeoverview.htm), and
// Block Volume (https://docs.cloud.oracle.com/iaas/Content/Block/Concepts/overview.htm) services. Use this API
// to manage resources such as virtual cloud networks (VCNs), compute instances, and
// block storage volumes.
//

package core

import (
	"github.com/oracle/oci-go-sdk/common"
)

// VolumeBackupSchedule Defines the backup frequency and retention period for a volume backup policy. For more information,
// see Policy-Based Backups (https://docs.cloud.oracle.com/iaas/Content/Block/Tasks/schedulingvolumebackups.htm).
type VolumeBackupSchedule struct {

	// The type of volume backup to create.
	BackupType VolumeBackupScheduleBackupTypeEnum `mandatory:"true" json:"backupType"`

	// The volume backup frequency.
	Period VolumeBackupSchedulePeriodEnum `mandatory:"true" json:"period"`

	// How long, in seconds, to keep the volume backups created by this schedule.
	RetentionSeconds *int `mandatory:"true" json:"retentionSeconds"`

	// The number of seconds that the volume backup start time should be shifted from the default interval boundaries specified by the period. The volume backup start time is the frequency start time plus the offset.
	OffsetSeconds *int `mandatory:"false" json:"offsetSeconds"`

	// Indicates how the offset is defined. If value is `STRUCTURED`, then `hourOfDay`, `dayOfWeek`, `dayOfMonth`, and `month` fields are used and `offsetSeconds` will be ignored in requests and users should ignore its value from the responses.
	// `hourOfDay` is applicable for periods `ONE_DAY`, `ONE_WEEK`, `ONE_MONTH` and `ONE_YEAR`.
	// `dayOfWeek` is applicable for period `ONE_WEEK`.
	// `dayOfMonth` is applicable for periods `ONE_MONTH` and `ONE_YEAR`.
	// 'month' is applicable for period 'ONE_YEAR'.
	// They will be ignored in the requests for inapplicable periods.
	// If value is `NUMERIC_SECONDS`, then `offsetSeconds` will be used for both requests and responses and the structured fields will be ignored in the requests and users should ignore their values from the responses.
	// For clients using older versions of Apis and not sending `offsetType` in their requests, the behaviour is just like `NUMERIC_SECONDS`.
	OffsetType VolumeBackupScheduleOffsetTypeEnum `mandatory:"false" json:"offsetType,omitempty"`

	// The hour of the day to schedule the volume backup.
	HourOfDay *int `mandatory:"false" json:"hourOfDay"`

	// The day of the week to schedule the volume backup.
	DayOfWeek VolumeBackupScheduleDayOfWeekEnum `mandatory:"false" json:"dayOfWeek,omitempty"`

	// The day of the month to schedule the volume backup.
	DayOfMonth *int `mandatory:"false" json:"dayOfMonth"`

	// The month of the year to schedule the volume backup.
	Month VolumeBackupScheduleMonthEnum `mandatory:"false" json:"month,omitempty"`

	// Specifies what time zone is the schedule in
	TimeZone VolumeBackupScheduleTimeZoneEnum `mandatory:"false" json:"timeZone,omitempty"`
}

func (m VolumeBackupSchedule) String() string {
	return common.PointerString(m)
}

// VolumeBackupScheduleBackupTypeEnum Enum with underlying type: string
type VolumeBackupScheduleBackupTypeEnum string

// Set of constants representing the allowable values for VolumeBackupScheduleBackupTypeEnum
const (
	VolumeBackupScheduleBackupTypeFull        VolumeBackupScheduleBackupTypeEnum = "FULL"
	VolumeBackupScheduleBackupTypeIncremental VolumeBackupScheduleBackupTypeEnum = "INCREMENTAL"
)

var mappingVolumeBackupScheduleBackupType = map[string]VolumeBackupScheduleBackupTypeEnum{
	"FULL":        VolumeBackupScheduleBackupTypeFull,
	"INCREMENTAL": VolumeBackupScheduleBackupTypeIncremental,
}

// GetVolumeBackupScheduleBackupTypeEnumValues Enumerates the set of values for VolumeBackupScheduleBackupTypeEnum
func GetVolumeBackupScheduleBackupTypeEnumValues() []VolumeBackupScheduleBackupTypeEnum {
	values := make([]VolumeBackupScheduleBackupTypeEnum, 0)
	for _, v := range mappingVolumeBackupScheduleBackupType {
		values = append(values, v)
	}
	return values
}

// VolumeBackupSchedulePeriodEnum Enum with underlying type: string
type VolumeBackupSchedulePeriodEnum string

// Set of constants representing the allowable values for VolumeBackupSchedulePeriodEnum
const (
	VolumeBackupSchedulePeriodHour  VolumeBackupSchedulePeriodEnum = "ONE_HOUR"
	VolumeBackupSchedulePeriodDay   VolumeBackupSchedulePeriodEnum = "ONE_DAY"
	VolumeBackupSchedulePeriodWeek  VolumeBackupSchedulePeriodEnum = "ONE_WEEK"
	VolumeBackupSchedulePeriodMonth VolumeBackupSchedulePeriodEnum = "ONE_MONTH"
	VolumeBackupSchedulePeriodYear  VolumeBackupSchedulePeriodEnum = "ONE_YEAR"
)

var mappingVolumeBackupSchedulePeriod = map[string]VolumeBackupSchedulePeriodEnum{
	"ONE_HOUR":  VolumeBackupSchedulePeriodHour,
	"ONE_DAY":   VolumeBackupSchedulePeriodDay,
	"ONE_WEEK":  VolumeBackupSchedulePeriodWeek,
	"ONE_MONTH": VolumeBackupSchedulePeriodMonth,
	"ONE_YEAR":  VolumeBackupSchedulePeriodYear,
}

// GetVolumeBackupSchedulePeriodEnumValues Enumerates the set of values for VolumeBackupSchedulePeriodEnum
func GetVolumeBackupSchedulePeriodEnumValues() []VolumeBackupSchedulePeriodEnum {
	values := make([]VolumeBackupSchedulePeriodEnum, 0)
	for _, v := range mappingVolumeBackupSchedulePeriod {
		values = append(values, v)
	}
	return values
}

// VolumeBackupScheduleOffsetTypeEnum Enum with underlying type: string
type VolumeBackupScheduleOffsetTypeEnum string

// Set of constants representing the allowable values for VolumeBackupScheduleOffsetTypeEnum
const (
	VolumeBackupScheduleOffsetTypeStructured     VolumeBackupScheduleOffsetTypeEnum = "STRUCTURED"
	VolumeBackupScheduleOffsetTypeNumericSeconds VolumeBackupScheduleOffsetTypeEnum = "NUMERIC_SECONDS"
)

var mappingVolumeBackupScheduleOffsetType = map[string]VolumeBackupScheduleOffsetTypeEnum{
	"STRUCTURED":      VolumeBackupScheduleOffsetTypeStructured,
	"NUMERIC_SECONDS": VolumeBackupScheduleOffsetTypeNumericSeconds,
}

// GetVolumeBackupScheduleOffsetTypeEnumValues Enumerates the set of values for VolumeBackupScheduleOffsetTypeEnum
func GetVolumeBackupScheduleOffsetTypeEnumValues() []VolumeBackupScheduleOffsetTypeEnum {
	values := make([]VolumeBackupScheduleOffsetTypeEnum, 0)
	for _, v := range mappingVolumeBackupScheduleOffsetType {
		values = append(values, v)
	}
	return values
}

// VolumeBackupScheduleDayOfWeekEnum Enum with underlying type: string
type VolumeBackupScheduleDayOfWeekEnum string

// Set of constants representing the allowable values for VolumeBackupScheduleDayOfWeekEnum
const (
	VolumeBackupScheduleDayOfWeekMonday    VolumeBackupScheduleDayOfWeekEnum = "MONDAY"
	VolumeBackupScheduleDayOfWeekTuesday   VolumeBackupScheduleDayOfWeekEnum = "TUESDAY"
	VolumeBackupScheduleDayOfWeekWednesday VolumeBackupScheduleDayOfWeekEnum = "WEDNESDAY"
	VolumeBackupScheduleDayOfWeekThursday  VolumeBackupScheduleDayOfWeekEnum = "THURSDAY"
	VolumeBackupScheduleDayOfWeekFriday    VolumeBackupScheduleDayOfWeekEnum = "FRIDAY"
	VolumeBackupScheduleDayOfWeekSaturday  VolumeBackupScheduleDayOfWeekEnum = "SATURDAY"
	VolumeBackupScheduleDayOfWeekSunday    VolumeBackupScheduleDayOfWeekEnum = "SUNDAY"
)

var mappingVolumeBackupScheduleDayOfWeek = map[string]VolumeBackupScheduleDayOfWeekEnum{
	"MONDAY":    VolumeBackupScheduleDayOfWeekMonday,
	"TUESDAY":   VolumeBackupScheduleDayOfWeekTuesday,
	"WEDNESDAY": VolumeBackupScheduleDayOfWeekWednesday,
	"THURSDAY":  VolumeBackupScheduleDayOfWeekThursday,
	"FRIDAY":    VolumeBackupScheduleDayOfWeekFriday,
	"SATURDAY":  VolumeBackupScheduleDayOfWeekSaturday,
	"SUNDAY":    VolumeBackupScheduleDayOfWeekSunday,
}

// GetVolumeBackupScheduleDayOfWeekEnumValues Enumerates the set of values for VolumeBackupScheduleDayOfWeekEnum
func GetVolumeBackupScheduleDayOfWeekEnumValues() []VolumeBackupScheduleDayOfWeekEnum {
	values := make([]VolumeBackupScheduleDayOfWeekEnum, 0)
	for _, v := range mappingVolumeBackupScheduleDayOfWeek {
		values = append(values, v)
	}
	return values
}

// VolumeBackupScheduleMonthEnum Enum with underlying type: string
type VolumeBackupScheduleMonthEnum string

// Set of constants representing the allowable values for VolumeBackupScheduleMonthEnum
const (
	VolumeBackupScheduleMonthJanuary   VolumeBackupScheduleMonthEnum = "JANUARY"
	VolumeBackupScheduleMonthFebruary  VolumeBackupScheduleMonthEnum = "FEBRUARY"
	VolumeBackupScheduleMonthMarch     VolumeBackupScheduleMonthEnum = "MARCH"
	VolumeBackupScheduleMonthApril     VolumeBackupScheduleMonthEnum = "APRIL"
	VolumeBackupScheduleMonthMay       VolumeBackupScheduleMonthEnum = "MAY"
	VolumeBackupScheduleMonthJune      VolumeBackupScheduleMonthEnum = "JUNE"
	VolumeBackupScheduleMonthJuly      VolumeBackupScheduleMonthEnum = "JULY"
	VolumeBackupScheduleMonthAugust    VolumeBackupScheduleMonthEnum = "AUGUST"
	VolumeBackupScheduleMonthSeptember VolumeBackupScheduleMonthEnum = "SEPTEMBER"
	VolumeBackupScheduleMonthOctober   VolumeBackupScheduleMonthEnum = "OCTOBER"
	VolumeBackupScheduleMonthNovember  VolumeBackupScheduleMonthEnum = "NOVEMBER"
	VolumeBackupScheduleMonthDecember  VolumeBackupScheduleMonthEnum = "DECEMBER"
)

var mappingVolumeBackupScheduleMonth = map[string]VolumeBackupScheduleMonthEnum{
	"JANUARY":   VolumeBackupScheduleMonthJanuary,
	"FEBRUARY":  VolumeBackupScheduleMonthFebruary,
	"MARCH":     VolumeBackupScheduleMonthMarch,
	"APRIL":     VolumeBackupScheduleMonthApril,
	"MAY":       VolumeBackupScheduleMonthMay,
	"JUNE":      VolumeBackupScheduleMonthJune,
	"JULY":      VolumeBackupScheduleMonthJuly,
	"AUGUST":    VolumeBackupScheduleMonthAugust,
	"SEPTEMBER": VolumeBackupScheduleMonthSeptember,
	"OCTOBER":   VolumeBackupScheduleMonthOctober,
	"NOVEMBER":  VolumeBackupScheduleMonthNovember,
	"DECEMBER":  VolumeBackupScheduleMonthDecember,
}

// GetVolumeBackupScheduleMonthEnumValues Enumerates the set of values for VolumeBackupScheduleMonthEnum
func GetVolumeBackupScheduleMonthEnumValues() []VolumeBackupScheduleMonthEnum {
	values := make([]VolumeBackupScheduleMonthEnum, 0)
	for _, v := range mappingVolumeBackupScheduleMonth {
		values = append(values, v)
	}
	return values
}

// VolumeBackupScheduleTimeZoneEnum Enum with underlying type: string
type VolumeBackupScheduleTimeZoneEnum string

// Set of constants representing the allowable values for VolumeBackupScheduleTimeZoneEnum
const (
	VolumeBackupScheduleTimeZoneUtc                    VolumeBackupScheduleTimeZoneEnum = "UTC"
	VolumeBackupScheduleTimeZoneRegionalDataCenterTime VolumeBackupScheduleTimeZoneEnum = "REGIONAL_DATA_CENTER_TIME"
)

var mappingVolumeBackupScheduleTimeZone = map[string]VolumeBackupScheduleTimeZoneEnum{
	"UTC":                       VolumeBackupScheduleTimeZoneUtc,
	"REGIONAL_DATA_CENTER_TIME": VolumeBackupScheduleTimeZoneRegionalDataCenterTime,
}

// GetVolumeBackupScheduleTimeZoneEnumValues Enumerates the set of values for VolumeBackupScheduleTimeZoneEnum
func GetVolumeBackupScheduleTimeZoneEnumValues() []VolumeBackupScheduleTimeZoneEnum {
	values := make([]VolumeBackupScheduleTimeZoneEnum, 0)
	for _, v := range mappingVolumeBackupScheduleTimeZone {
		values = append(values, v)
	}
	return values
}
