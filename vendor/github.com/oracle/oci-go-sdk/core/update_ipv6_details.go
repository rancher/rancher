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

// UpdateIpv6Details The representation of UpdateIpv6Details
type UpdateIpv6Details struct {

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable. Avoid
	// entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// Whether the IPv6 can be used for internet communication. Allowed by default for an IPv6 in
	// a public subnet. Never allowed for an IPv6 in a private subnet. If the value is `true`, the
	// IPv6 uses its public IP address for internet communication.
	// If you switch this from `true` to `false`, the `publicIpAddress` attribute for the IPv6
	// becomes null.
	// Example: `false`
	IsInternetAccessAllowed *bool `mandatory:"false" json:"isInternetAccessAllowed"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the VNIC to reassign the IPv6 to.
	// The VNIC must be in the same subnet as the current VNIC.
	VnicId *string `mandatory:"false" json:"vnicId"`
}

func (m UpdateIpv6Details) String() string {
	return common.PointerString(m)
}
