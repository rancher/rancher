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

// WorkRequestOperationTypeEnum Enum with underlying type: string
type WorkRequestOperationTypeEnum string

// Set of constants representing the allowable values for WorkRequestOperationTypeEnum
const (
	WorkRequestOperationTypeClusterCreate     WorkRequestOperationTypeEnum = "CLUSTER_CREATE"
	WorkRequestOperationTypeClusterUpdate     WorkRequestOperationTypeEnum = "CLUSTER_UPDATE"
	WorkRequestOperationTypeClusterDelete     WorkRequestOperationTypeEnum = "CLUSTER_DELETE"
	WorkRequestOperationTypeNodepoolCreate    WorkRequestOperationTypeEnum = "NODEPOOL_CREATE"
	WorkRequestOperationTypeNodepoolUpdate    WorkRequestOperationTypeEnum = "NODEPOOL_UPDATE"
	WorkRequestOperationTypeNodepoolDelete    WorkRequestOperationTypeEnum = "NODEPOOL_DELETE"
	WorkRequestOperationTypeWorkrequestCancel WorkRequestOperationTypeEnum = "WORKREQUEST_CANCEL"
)

var mappingWorkRequestOperationType = map[string]WorkRequestOperationTypeEnum{
	"CLUSTER_CREATE":     WorkRequestOperationTypeClusterCreate,
	"CLUSTER_UPDATE":     WorkRequestOperationTypeClusterUpdate,
	"CLUSTER_DELETE":     WorkRequestOperationTypeClusterDelete,
	"NODEPOOL_CREATE":    WorkRequestOperationTypeNodepoolCreate,
	"NODEPOOL_UPDATE":    WorkRequestOperationTypeNodepoolUpdate,
	"NODEPOOL_DELETE":    WorkRequestOperationTypeNodepoolDelete,
	"WORKREQUEST_CANCEL": WorkRequestOperationTypeWorkrequestCancel,
}

// GetWorkRequestOperationTypeEnumValues Enumerates the set of values for WorkRequestOperationTypeEnum
func GetWorkRequestOperationTypeEnumValues() []WorkRequestOperationTypeEnum {
	values := make([]WorkRequestOperationTypeEnum, 0)
	for _, v := range mappingWorkRequestOperationType {
		values = append(values, v)
	}
	return values
}
