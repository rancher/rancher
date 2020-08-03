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

// CreateInstancePoolPlacementConfigurationDetails The location for where an instance pool will place instances.
type CreateInstancePoolPlacementConfigurationDetails struct {

	// The availability domain to place instances.
	// Example: `Uocm:PHX-AD-1`
	AvailabilityDomain *string `mandatory:"true" json:"availabilityDomain"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the primary subnet to place instances.
	PrimarySubnetId *string `mandatory:"true" json:"primarySubnetId"`

	// The fault domains to place instances.
	// If you don't provide any values, the system makes a best effort to distribute
	// instances across all fault domains based on capacity.
	// To distribute the instances evenly across selected fault domains, provide a
	// set of fault domains. For example, you might want instances to be evenly
	// distributed if your applications require high availability.
	// To get a list of fault domains, use the
	// ListFaultDomains operation
	// in the Identity and Access Management Service API.
	// Example: `[FAULT-DOMAIN-1, FAULT-DOMAIN-2, FAULT-DOMAIN-3]`
	FaultDomains []string `mandatory:"false" json:"faultDomains"`

	// The set of secondary VNIC data for instances in the pool.
	SecondaryVnicSubnets []InstancePoolPlacementSecondaryVnicSubnet `mandatory:"false" json:"secondaryVnicSubnets"`
}

func (m CreateInstancePoolPlacementConfigurationDetails) String() string {
	return common.PointerString(m)
}
