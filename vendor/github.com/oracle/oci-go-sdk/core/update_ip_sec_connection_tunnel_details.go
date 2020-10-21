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

// UpdateIpSecConnectionTunnelDetails The representation of UpdateIpSecConnectionTunnelDetails
type UpdateIpSecConnectionTunnelDetails struct {

	// A user-friendly name. Does not have to be unique, and it's changeable. Avoid
	// entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// The type of routing to use for this tunnel (either BGP dynamic routing or static routing).
	Routing UpdateIpSecConnectionTunnelDetailsRoutingEnum `mandatory:"false" json:"routing,omitempty"`

	// Internet Key Exchange protocol version.
	IkeVersion UpdateIpSecConnectionTunnelDetailsIkeVersionEnum `mandatory:"false" json:"ikeVersion,omitempty"`

	// Information for establishing a BGP session for the IPSec tunnel.
	BgpSessionConfig *UpdateIpSecTunnelBgpSessionDetails `mandatory:"false" json:"bgpSessionConfig"`
}

func (m UpdateIpSecConnectionTunnelDetails) String() string {
	return common.PointerString(m)
}

// UpdateIpSecConnectionTunnelDetailsRoutingEnum Enum with underlying type: string
type UpdateIpSecConnectionTunnelDetailsRoutingEnum string

// Set of constants representing the allowable values for UpdateIpSecConnectionTunnelDetailsRoutingEnum
const (
	UpdateIpSecConnectionTunnelDetailsRoutingBgp    UpdateIpSecConnectionTunnelDetailsRoutingEnum = "BGP"
	UpdateIpSecConnectionTunnelDetailsRoutingStatic UpdateIpSecConnectionTunnelDetailsRoutingEnum = "STATIC"
)

var mappingUpdateIpSecConnectionTunnelDetailsRouting = map[string]UpdateIpSecConnectionTunnelDetailsRoutingEnum{
	"BGP":    UpdateIpSecConnectionTunnelDetailsRoutingBgp,
	"STATIC": UpdateIpSecConnectionTunnelDetailsRoutingStatic,
}

// GetUpdateIpSecConnectionTunnelDetailsRoutingEnumValues Enumerates the set of values for UpdateIpSecConnectionTunnelDetailsRoutingEnum
func GetUpdateIpSecConnectionTunnelDetailsRoutingEnumValues() []UpdateIpSecConnectionTunnelDetailsRoutingEnum {
	values := make([]UpdateIpSecConnectionTunnelDetailsRoutingEnum, 0)
	for _, v := range mappingUpdateIpSecConnectionTunnelDetailsRouting {
		values = append(values, v)
	}
	return values
}

// UpdateIpSecConnectionTunnelDetailsIkeVersionEnum Enum with underlying type: string
type UpdateIpSecConnectionTunnelDetailsIkeVersionEnum string

// Set of constants representing the allowable values for UpdateIpSecConnectionTunnelDetailsIkeVersionEnum
const (
	UpdateIpSecConnectionTunnelDetailsIkeVersionV1 UpdateIpSecConnectionTunnelDetailsIkeVersionEnum = "V1"
	UpdateIpSecConnectionTunnelDetailsIkeVersionV2 UpdateIpSecConnectionTunnelDetailsIkeVersionEnum = "V2"
)

var mappingUpdateIpSecConnectionTunnelDetailsIkeVersion = map[string]UpdateIpSecConnectionTunnelDetailsIkeVersionEnum{
	"V1": UpdateIpSecConnectionTunnelDetailsIkeVersionV1,
	"V2": UpdateIpSecConnectionTunnelDetailsIkeVersionV2,
}

// GetUpdateIpSecConnectionTunnelDetailsIkeVersionEnumValues Enumerates the set of values for UpdateIpSecConnectionTunnelDetailsIkeVersionEnum
func GetUpdateIpSecConnectionTunnelDetailsIkeVersionEnumValues() []UpdateIpSecConnectionTunnelDetailsIkeVersionEnum {
	values := make([]UpdateIpSecConnectionTunnelDetailsIkeVersionEnum, 0)
	for _, v := range mappingUpdateIpSecConnectionTunnelDetailsIkeVersion {
		values = append(values, v)
	}
	return values
}
