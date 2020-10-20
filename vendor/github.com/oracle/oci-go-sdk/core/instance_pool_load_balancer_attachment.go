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

// InstancePoolLoadBalancerAttachment Represents a load balancer that is attached to an instance pool.
type InstancePoolLoadBalancerAttachment struct {

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the load balancer attachment.
	Id *string `mandatory:"true" json:"id"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the instance pool of the load balancer attachment.
	InstancePoolId *string `mandatory:"true" json:"instancePoolId"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the load balancer attached to the instance pool.
	LoadBalancerId *string `mandatory:"true" json:"loadBalancerId"`

	// The name of the backend set on the load balancer.
	BackendSetName *string `mandatory:"true" json:"backendSetName"`

	// The port value used for the backends.
	Port *int `mandatory:"true" json:"port"`

	// Indicates which VNIC on each instance in the instance pool should be used to associate with the load balancer. Possible values are "PrimaryVnic" or the displayName of one of the secondary VNICs on the instance configuration that is associated with the instance pool.
	VnicSelection *string `mandatory:"true" json:"vnicSelection"`

	// The status of the interaction between the instance pool and the load balancer.
	LifecycleState InstancePoolLoadBalancerAttachmentLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`
}

func (m InstancePoolLoadBalancerAttachment) String() string {
	return common.PointerString(m)
}

// InstancePoolLoadBalancerAttachmentLifecycleStateEnum Enum with underlying type: string
type InstancePoolLoadBalancerAttachmentLifecycleStateEnum string

// Set of constants representing the allowable values for InstancePoolLoadBalancerAttachmentLifecycleStateEnum
const (
	InstancePoolLoadBalancerAttachmentLifecycleStateAttaching InstancePoolLoadBalancerAttachmentLifecycleStateEnum = "ATTACHING"
	InstancePoolLoadBalancerAttachmentLifecycleStateAttached  InstancePoolLoadBalancerAttachmentLifecycleStateEnum = "ATTACHED"
	InstancePoolLoadBalancerAttachmentLifecycleStateDetaching InstancePoolLoadBalancerAttachmentLifecycleStateEnum = "DETACHING"
	InstancePoolLoadBalancerAttachmentLifecycleStateDetached  InstancePoolLoadBalancerAttachmentLifecycleStateEnum = "DETACHED"
)

var mappingInstancePoolLoadBalancerAttachmentLifecycleState = map[string]InstancePoolLoadBalancerAttachmentLifecycleStateEnum{
	"ATTACHING": InstancePoolLoadBalancerAttachmentLifecycleStateAttaching,
	"ATTACHED":  InstancePoolLoadBalancerAttachmentLifecycleStateAttached,
	"DETACHING": InstancePoolLoadBalancerAttachmentLifecycleStateDetaching,
	"DETACHED":  InstancePoolLoadBalancerAttachmentLifecycleStateDetached,
}

// GetInstancePoolLoadBalancerAttachmentLifecycleStateEnumValues Enumerates the set of values for InstancePoolLoadBalancerAttachmentLifecycleStateEnum
func GetInstancePoolLoadBalancerAttachmentLifecycleStateEnumValues() []InstancePoolLoadBalancerAttachmentLifecycleStateEnum {
	values := make([]InstancePoolLoadBalancerAttachmentLifecycleStateEnum, 0)
	for _, v := range mappingInstancePoolLoadBalancerAttachmentLifecycleState {
		values = append(values, v)
	}
	return values
}
