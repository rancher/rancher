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

// DedicatedVmHostSummary A dedicated virtual machine (VM) host that enables you to host multiple instances on a dedicated host instance that is not shared with other tenancies.
type DedicatedVmHostSummary struct {

	// The availability domain the dedicated VM host is running in.
	// Example: `Uocm:PHX-AD-1`
	AvailabilityDomain *string `mandatory:"true" json:"availabilityDomain"`

	// The OCID of the compartment that contains the dedicated VM host.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The shape of the dedicated VM host. The shape determines the number of CPUs and
	// other resources available for VMs.
	DedicatedVmHostShape *string `mandatory:"true" json:"dedicatedVmHostShape"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	// Example: `My DVH`
	DisplayName *string `mandatory:"true" json:"displayName"`

	// The OCID of the dedicated VM host.
	Id *string `mandatory:"true" json:"id"`

	// The current state of the dedicated VM host.
	LifecycleState DedicatedVmHostSummaryLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// The date and time the dedicated VM host was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// The current available OCPUs of the dedicated VM host.
	RemainingOcpus *float32 `mandatory:"true" json:"remainingOcpus"`

	// The current total OCPUs of the dedicated VM host.
	TotalOcpus *float32 `mandatory:"true" json:"totalOcpus"`

	// The fault domain for the dedicated VM host's assigned instances. For more information, see Fault Domains.
	// If you do not specify the fault domain, the system selects one for you. To change the fault domain for a dedicated VM host,
	// delete it and create a new dedicated VM host in the preferred fault domain.
	// To get a list of fault domains, use the ListFaultDomains operation in the Identity and Access Management Service API.
	// Example: `FAULT-DOMAIN-1`
	FaultDomain *string `mandatory:"false" json:"faultDomain"`
}

func (m DedicatedVmHostSummary) String() string {
	return common.PointerString(m)
}

// DedicatedVmHostSummaryLifecycleStateEnum Enum with underlying type: string
type DedicatedVmHostSummaryLifecycleStateEnum string

// Set of constants representing the allowable values for DedicatedVmHostSummaryLifecycleStateEnum
const (
	DedicatedVmHostSummaryLifecycleStateCreating DedicatedVmHostSummaryLifecycleStateEnum = "CREATING"
	DedicatedVmHostSummaryLifecycleStateActive   DedicatedVmHostSummaryLifecycleStateEnum = "ACTIVE"
	DedicatedVmHostSummaryLifecycleStateUpdating DedicatedVmHostSummaryLifecycleStateEnum = "UPDATING"
	DedicatedVmHostSummaryLifecycleStateDeleting DedicatedVmHostSummaryLifecycleStateEnum = "DELETING"
	DedicatedVmHostSummaryLifecycleStateDeleted  DedicatedVmHostSummaryLifecycleStateEnum = "DELETED"
	DedicatedVmHostSummaryLifecycleStateFailed   DedicatedVmHostSummaryLifecycleStateEnum = "FAILED"
)

var mappingDedicatedVmHostSummaryLifecycleState = map[string]DedicatedVmHostSummaryLifecycleStateEnum{
	"CREATING": DedicatedVmHostSummaryLifecycleStateCreating,
	"ACTIVE":   DedicatedVmHostSummaryLifecycleStateActive,
	"UPDATING": DedicatedVmHostSummaryLifecycleStateUpdating,
	"DELETING": DedicatedVmHostSummaryLifecycleStateDeleting,
	"DELETED":  DedicatedVmHostSummaryLifecycleStateDeleted,
	"FAILED":   DedicatedVmHostSummaryLifecycleStateFailed,
}

// GetDedicatedVmHostSummaryLifecycleStateEnumValues Enumerates the set of values for DedicatedVmHostSummaryLifecycleStateEnum
func GetDedicatedVmHostSummaryLifecycleStateEnumValues() []DedicatedVmHostSummaryLifecycleStateEnum {
	values := make([]DedicatedVmHostSummaryLifecycleStateEnum, 0)
	for _, v := range mappingDedicatedVmHostSummaryLifecycleState {
		values = append(values, v)
	}
	return values
}
