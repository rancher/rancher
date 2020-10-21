// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Identity and Access Management Service API
//
// APIs for managing users, groups, compartments, and policies.
//

package identity

import (
	"github.com/oracle/oci-go-sdk/common"
)

// TaggingWorkRequestSummary The work request summary. Tracks the status of the asynchronous operation.
type TaggingWorkRequestSummary struct {

	// The OCID of the work request.
	Id *string `mandatory:"true" json:"id"`

	// An enum-like description of the type of work the work request is doing.
	OperationType TaggingWorkRequestSummaryOperationTypeEnum `mandatory:"true" json:"operationType"`

	// The current status of the work request.
	Status TaggingWorkRequestSummaryStatusEnum `mandatory:"true" json:"status"`

	// The OCID of the compartment that contains the work request.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// The resources this work request affects.
	Resources []WorkRequestResource `mandatory:"false" json:"resources"`

	// Date and time the work was accepted, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeAccepted *common.SDKTime `mandatory:"false" json:"timeAccepted"`

	// Date and time the work started, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeStarted *common.SDKTime `mandatory:"false" json:"timeStarted"`

	// Date and time the work completed, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeFinished *common.SDKTime `mandatory:"false" json:"timeFinished"`

	// How much progress the operation has made.
	PercentComplete *float32 `mandatory:"false" json:"percentComplete"`
}

func (m TaggingWorkRequestSummary) String() string {
	return common.PointerString(m)
}

// TaggingWorkRequestSummaryOperationTypeEnum Enum with underlying type: string
type TaggingWorkRequestSummaryOperationTypeEnum string

// Set of constants representing the allowable values for TaggingWorkRequestSummaryOperationTypeEnum
const (
	TaggingWorkRequestSummaryOperationTypeDeleteTagDefinition TaggingWorkRequestSummaryOperationTypeEnum = "DELETE_TAG_DEFINITION"
)

var mappingTaggingWorkRequestSummaryOperationType = map[string]TaggingWorkRequestSummaryOperationTypeEnum{
	"DELETE_TAG_DEFINITION": TaggingWorkRequestSummaryOperationTypeDeleteTagDefinition,
}

// GetTaggingWorkRequestSummaryOperationTypeEnumValues Enumerates the set of values for TaggingWorkRequestSummaryOperationTypeEnum
func GetTaggingWorkRequestSummaryOperationTypeEnumValues() []TaggingWorkRequestSummaryOperationTypeEnum {
	values := make([]TaggingWorkRequestSummaryOperationTypeEnum, 0)
	for _, v := range mappingTaggingWorkRequestSummaryOperationType {
		values = append(values, v)
	}
	return values
}

// TaggingWorkRequestSummaryStatusEnum Enum with underlying type: string
type TaggingWorkRequestSummaryStatusEnum string

// Set of constants representing the allowable values for TaggingWorkRequestSummaryStatusEnum
const (
	TaggingWorkRequestSummaryStatusAccepted   TaggingWorkRequestSummaryStatusEnum = "ACCEPTED"
	TaggingWorkRequestSummaryStatusInProgress TaggingWorkRequestSummaryStatusEnum = "IN_PROGRESS"
	TaggingWorkRequestSummaryStatusFailed     TaggingWorkRequestSummaryStatusEnum = "FAILED"
	TaggingWorkRequestSummaryStatusSucceeded  TaggingWorkRequestSummaryStatusEnum = "SUCCEEDED"
	TaggingWorkRequestSummaryStatusCanceling  TaggingWorkRequestSummaryStatusEnum = "CANCELING"
	TaggingWorkRequestSummaryStatusCanceled   TaggingWorkRequestSummaryStatusEnum = "CANCELED"
)

var mappingTaggingWorkRequestSummaryStatus = map[string]TaggingWorkRequestSummaryStatusEnum{
	"ACCEPTED":    TaggingWorkRequestSummaryStatusAccepted,
	"IN_PROGRESS": TaggingWorkRequestSummaryStatusInProgress,
	"FAILED":      TaggingWorkRequestSummaryStatusFailed,
	"SUCCEEDED":   TaggingWorkRequestSummaryStatusSucceeded,
	"CANCELING":   TaggingWorkRequestSummaryStatusCanceling,
	"CANCELED":    TaggingWorkRequestSummaryStatusCanceled,
}

// GetTaggingWorkRequestSummaryStatusEnumValues Enumerates the set of values for TaggingWorkRequestSummaryStatusEnum
func GetTaggingWorkRequestSummaryStatusEnumValues() []TaggingWorkRequestSummaryStatusEnum {
	values := make([]TaggingWorkRequestSummaryStatusEnum, 0)
	for _, v := range mappingTaggingWorkRequestSummaryStatus {
		values = append(values, v)
	}
	return values
}
