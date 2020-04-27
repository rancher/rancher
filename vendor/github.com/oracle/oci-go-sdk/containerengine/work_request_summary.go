// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Container Engine for Kubernetes API
//
// API for the Container Engine for Kubernetes service. Use this API to build, deploy,
// and manage cloud-native applications. For more information, see
// Overview of Container Engine for Kubernetes (https://docs.cloud.oracle.com/iaas/Content/ContEng/Concepts/contengoverview.htm).
//

package containerengine

import (
	"github.com/oracle/oci-go-sdk/common"
)

// WorkRequestSummary The properties that define a work request summary.
type WorkRequestSummary struct {

	// The OCID of the work request.
	Id *string `mandatory:"false" json:"id"`

	// The type of work the work request is doing.
	OperationType WorkRequestOperationTypeEnum `mandatory:"false" json:"operationType,omitempty"`

	// The current status of the work request.
	Status WorkRequestStatusEnum `mandatory:"false" json:"status,omitempty"`

	// The OCID of the compartment in which the work request exists.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// The resources this work request affects.
	Resources []WorkRequestResource `mandatory:"false" json:"resources"`

	// The time the work request was accepted.
	TimeAccepted *common.SDKTime `mandatory:"false" json:"timeAccepted"`

	// The time the work request was started.
	TimeStarted *common.SDKTime `mandatory:"false" json:"timeStarted"`

	// The time the work request was finished.
	TimeFinished *common.SDKTime `mandatory:"false" json:"timeFinished"`
}

func (m WorkRequestSummary) String() string {
	return common.PointerString(m)
}

// WorkRequestSummaryOperationTypeEnum is an alias to type: WorkRequestOperationTypeEnum
// Consider using WorkRequestOperationTypeEnum instead
// Deprecated
type WorkRequestSummaryOperationTypeEnum = WorkRequestOperationTypeEnum

// Set of constants representing the allowable values for WorkRequestOperationTypeEnum
// Deprecated
const (
	WorkRequestSummaryOperationTypeClusterCreate     WorkRequestOperationTypeEnum = "CLUSTER_CREATE"
	WorkRequestSummaryOperationTypeClusterUpdate     WorkRequestOperationTypeEnum = "CLUSTER_UPDATE"
	WorkRequestSummaryOperationTypeClusterDelete     WorkRequestOperationTypeEnum = "CLUSTER_DELETE"
	WorkRequestSummaryOperationTypeNodepoolCreate    WorkRequestOperationTypeEnum = "NODEPOOL_CREATE"
	WorkRequestSummaryOperationTypeNodepoolUpdate    WorkRequestOperationTypeEnum = "NODEPOOL_UPDATE"
	WorkRequestSummaryOperationTypeNodepoolDelete    WorkRequestOperationTypeEnum = "NODEPOOL_DELETE"
	WorkRequestSummaryOperationTypeWorkrequestCancel WorkRequestOperationTypeEnum = "WORKREQUEST_CANCEL"
)

// GetWorkRequestSummaryOperationTypeEnumValues Enumerates the set of values for WorkRequestOperationTypeEnum
// Consider using GetWorkRequestOperationTypeEnumValue
// Deprecated
var GetWorkRequestSummaryOperationTypeEnumValues = GetWorkRequestOperationTypeEnumValues

// WorkRequestSummaryStatusEnum is an alias to type: WorkRequestStatusEnum
// Consider using WorkRequestStatusEnum instead
// Deprecated
type WorkRequestSummaryStatusEnum = WorkRequestStatusEnum

// Set of constants representing the allowable values for WorkRequestStatusEnum
// Deprecated
const (
	WorkRequestSummaryStatusAccepted   WorkRequestStatusEnum = "ACCEPTED"
	WorkRequestSummaryStatusInProgress WorkRequestStatusEnum = "IN_PROGRESS"
	WorkRequestSummaryStatusFailed     WorkRequestStatusEnum = "FAILED"
	WorkRequestSummaryStatusSucceeded  WorkRequestStatusEnum = "SUCCEEDED"
	WorkRequestSummaryStatusCanceling  WorkRequestStatusEnum = "CANCELING"
	WorkRequestSummaryStatusCanceled   WorkRequestStatusEnum = "CANCELED"
)

// GetWorkRequestSummaryStatusEnumValues Enumerates the set of values for WorkRequestStatusEnum
// Consider using GetWorkRequestStatusEnumValue
// Deprecated
var GetWorkRequestSummaryStatusEnumValues = GetWorkRequestStatusEnumValues
