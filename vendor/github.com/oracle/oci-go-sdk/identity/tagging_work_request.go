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

// TaggingWorkRequest The asynchronous API request does not take effect immediately. This request spawns an asynchronous
// workflow to fulfill the request. WorkRequest objects provide visibility for in-progress workflows.
type TaggingWorkRequest struct {

	// The OCID of the work request.
	Id *string `mandatory:"true" json:"id"`

	// An enum-like description of the type of work the work request is doing.
	OperationType TaggingWorkRequestOperationTypeEnum `mandatory:"true" json:"operationType"`

	// The current status of the work request.
	Status TaggingWorkRequestStatusEnum `mandatory:"true" json:"status"`

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

func (m TaggingWorkRequest) String() string {
	return common.PointerString(m)
}

// TaggingWorkRequestOperationTypeEnum Enum with underlying type: string
type TaggingWorkRequestOperationTypeEnum string

// Set of constants representing the allowable values for TaggingWorkRequestOperationTypeEnum
const (
	TaggingWorkRequestOperationTypeDeleteTagDefinition TaggingWorkRequestOperationTypeEnum = "DELETE_TAG_DEFINITION"
)

var mappingTaggingWorkRequestOperationType = map[string]TaggingWorkRequestOperationTypeEnum{
	"DELETE_TAG_DEFINITION": TaggingWorkRequestOperationTypeDeleteTagDefinition,
}

// GetTaggingWorkRequestOperationTypeEnumValues Enumerates the set of values for TaggingWorkRequestOperationTypeEnum
func GetTaggingWorkRequestOperationTypeEnumValues() []TaggingWorkRequestOperationTypeEnum {
	values := make([]TaggingWorkRequestOperationTypeEnum, 0)
	for _, v := range mappingTaggingWorkRequestOperationType {
		values = append(values, v)
	}
	return values
}

// TaggingWorkRequestStatusEnum Enum with underlying type: string
type TaggingWorkRequestStatusEnum string

// Set of constants representing the allowable values for TaggingWorkRequestStatusEnum
const (
	TaggingWorkRequestStatusAccepted   TaggingWorkRequestStatusEnum = "ACCEPTED"
	TaggingWorkRequestStatusInProgress TaggingWorkRequestStatusEnum = "IN_PROGRESS"
	TaggingWorkRequestStatusFailed     TaggingWorkRequestStatusEnum = "FAILED"
	TaggingWorkRequestStatusSucceeded  TaggingWorkRequestStatusEnum = "SUCCEEDED"
	TaggingWorkRequestStatusCanceling  TaggingWorkRequestStatusEnum = "CANCELING"
	TaggingWorkRequestStatusCanceled   TaggingWorkRequestStatusEnum = "CANCELED"
)

var mappingTaggingWorkRequestStatus = map[string]TaggingWorkRequestStatusEnum{
	"ACCEPTED":    TaggingWorkRequestStatusAccepted,
	"IN_PROGRESS": TaggingWorkRequestStatusInProgress,
	"FAILED":      TaggingWorkRequestStatusFailed,
	"SUCCEEDED":   TaggingWorkRequestStatusSucceeded,
	"CANCELING":   TaggingWorkRequestStatusCanceling,
	"CANCELED":    TaggingWorkRequestStatusCanceled,
}

// GetTaggingWorkRequestStatusEnumValues Enumerates the set of values for TaggingWorkRequestStatusEnum
func GetTaggingWorkRequestStatusEnumValues() []TaggingWorkRequestStatusEnum {
	values := make([]TaggingWorkRequestStatusEnum, 0)
	for _, v := range mappingTaggingWorkRequestStatus {
		values = append(values, v)
	}
	return values
}
