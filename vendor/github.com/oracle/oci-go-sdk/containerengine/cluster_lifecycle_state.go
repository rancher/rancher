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

// ClusterLifecycleStateEnum Enum with underlying type: string
type ClusterLifecycleStateEnum string

// Set of constants representing the allowable values for ClusterLifecycleStateEnum
const (
	ClusterLifecycleStateCreating ClusterLifecycleStateEnum = "CREATING"
	ClusterLifecycleStateActive   ClusterLifecycleStateEnum = "ACTIVE"
	ClusterLifecycleStateFailed   ClusterLifecycleStateEnum = "FAILED"
	ClusterLifecycleStateDeleting ClusterLifecycleStateEnum = "DELETING"
	ClusterLifecycleStateDeleted  ClusterLifecycleStateEnum = "DELETED"
	ClusterLifecycleStateUpdating ClusterLifecycleStateEnum = "UPDATING"
)

var mappingClusterLifecycleState = map[string]ClusterLifecycleStateEnum{
	"CREATING": ClusterLifecycleStateCreating,
	"ACTIVE":   ClusterLifecycleStateActive,
	"FAILED":   ClusterLifecycleStateFailed,
	"DELETING": ClusterLifecycleStateDeleting,
	"DELETED":  ClusterLifecycleStateDeleted,
	"UPDATING": ClusterLifecycleStateUpdating,
}

// GetClusterLifecycleStateEnumValues Enumerates the set of values for ClusterLifecycleStateEnum
func GetClusterLifecycleStateEnumValues() []ClusterLifecycleStateEnum {
	values := make([]ClusterLifecycleStateEnum, 0)
	for _, v := range mappingClusterLifecycleState {
		values = append(values, v)
	}
	return values
}
