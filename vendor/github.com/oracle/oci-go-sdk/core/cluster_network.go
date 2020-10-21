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

// ClusterNetwork A cluster network is a group of high performance computing (HPC) bare metal instances that are connected
// with an ultra low latency network. For more information about cluster networks, see
// Managing Cluster Networks (https://docs.cloud.oracle.com/iaas/Content/Compute/Tasks/managingclusternetworks.htm).
type ClusterNetwork struct {

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the cluster network.
	Id *string `mandatory:"true" json:"id"`

	// The OCID (https://docs.cloud.oracle.com/iaas/Content/General/Concepts/identifiers.htm) of the compartment containing the cluster netowrk.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The current state of the cluster network.
	LifecycleState ClusterNetworkLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// The date and time the resource was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The date and time the resource was updated, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeUpdated *common.SDKTime `mandatory:"true" json:"timeUpdated"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// The instance pools in the cluster network.
	// Each cluster network can have one instance pool.
	InstancePools []InstancePool `mandatory:"false" json:"instancePools"`

	// The placement configuration for the instance pools in the cluster network.
	PlacementConfiguration *ClusterNetworkPlacementConfigurationDetails `mandatory:"false" json:"placementConfiguration"`
}

func (m ClusterNetwork) String() string {
	return common.PointerString(m)
}

// ClusterNetworkLifecycleStateEnum Enum with underlying type: string
type ClusterNetworkLifecycleStateEnum string

// Set of constants representing the allowable values for ClusterNetworkLifecycleStateEnum
const (
	ClusterNetworkLifecycleStateProvisioning ClusterNetworkLifecycleStateEnum = "PROVISIONING"
	ClusterNetworkLifecycleStateScaling      ClusterNetworkLifecycleStateEnum = "SCALING"
	ClusterNetworkLifecycleStateStarting     ClusterNetworkLifecycleStateEnum = "STARTING"
	ClusterNetworkLifecycleStateStopping     ClusterNetworkLifecycleStateEnum = "STOPPING"
	ClusterNetworkLifecycleStateTerminating  ClusterNetworkLifecycleStateEnum = "TERMINATING"
	ClusterNetworkLifecycleStateStopped      ClusterNetworkLifecycleStateEnum = "STOPPED"
	ClusterNetworkLifecycleStateTerminated   ClusterNetworkLifecycleStateEnum = "TERMINATED"
	ClusterNetworkLifecycleStateRunning      ClusterNetworkLifecycleStateEnum = "RUNNING"
)

var mappingClusterNetworkLifecycleState = map[string]ClusterNetworkLifecycleStateEnum{
	"PROVISIONING": ClusterNetworkLifecycleStateProvisioning,
	"SCALING":      ClusterNetworkLifecycleStateScaling,
	"STARTING":     ClusterNetworkLifecycleStateStarting,
	"STOPPING":     ClusterNetworkLifecycleStateStopping,
	"TERMINATING":  ClusterNetworkLifecycleStateTerminating,
	"STOPPED":      ClusterNetworkLifecycleStateStopped,
	"TERMINATED":   ClusterNetworkLifecycleStateTerminated,
	"RUNNING":      ClusterNetworkLifecycleStateRunning,
}

// GetClusterNetworkLifecycleStateEnumValues Enumerates the set of values for ClusterNetworkLifecycleStateEnum
func GetClusterNetworkLifecycleStateEnumValues() []ClusterNetworkLifecycleStateEnum {
	values := make([]ClusterNetworkLifecycleStateEnum, 0)
	for _, v := range mappingClusterNetworkLifecycleState {
		values = append(values, v)
	}
	return values
}
