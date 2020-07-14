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

// NetworkSourcesSummary A network source defines a list of source IPs that are allowed to make auth requests
// More info needed here
type NetworkSourcesSummary struct {

	// The OCID of the network source.
	Id *string `mandatory:"false" json:"id"`

	// The OCID of the tenancy containing the network source.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// The name you assign to the network source during creation. The name must be unique across
	// the tenancy and cannot be changed.
	Name *string `mandatory:"false" json:"name"`

	// The description you assign to the network source. Does not have to be unique, and it's changeable.
	Description *string `mandatory:"false" json:"description"`

	// A list of allowed public IPs and CIDR ranges
	PublicSourceList []string `mandatory:"false" json:"publicSourceList"`

	// A list of allowed VCN ocid/IP range pairs
	VirtualSourceList []NetworkSourcesVirtualSourceList `mandatory:"false" json:"virtualSourceList"`

	// A list of OCIservices allowed to make on behalf of requests which may have different source ips.
	// At this time only the values of all or none are supported.
	Services []string `mandatory:"false" json:"services"`

	// Date and time the group was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`
}

func (m NetworkSourcesSummary) String() string {
	return common.PointerString(m)
}
