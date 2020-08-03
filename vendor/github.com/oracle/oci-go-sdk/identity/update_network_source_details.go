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

// UpdateNetworkSourceDetails The representation of UpdateNetworkSourceDetails
type UpdateNetworkSourceDetails struct {

	// The description you assign to the network source. Does not have to be unique, and it's changeable.
	Description *string `mandatory:"false" json:"description"`

	// A list of allowed public IPs and CIDR ranges
	PublicSourceList []string `mandatory:"false" json:"publicSourceList"`

	// A list of allowed VCN ocid/IP range pairs
	VirtualSourceList []NetworkSourcesVirtualSourceList `mandatory:"false" json:"virtualSourceList"`

	// A list of OCIservices allowed to make on behalf of requests which may have different source ips.
	// At this time only the values of all or none are supported.
	Services []string `mandatory:"false" json:"services"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no predefined name, type, or namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// Defined tags for this resource. Each key is predefined and scoped to a namespace.
	// For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`
}

func (m UpdateNetworkSourceDetails) String() string {
	return common.PointerString(m)
}
