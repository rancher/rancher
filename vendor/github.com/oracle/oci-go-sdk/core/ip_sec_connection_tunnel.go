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

// IpSecConnectionTunnel Information about a single tunnel in an IPSec connection. This object does not include the tunnel's
// shared secret (pre-shared key). That is in the
// IPSecConnectionTunnelSharedSecret object.
type IpSecConnectionTunnel struct {

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the compartment containing the tunnel.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the tunnel.
	Id *string `mandatory:"true" json:"id"`

	// The tunnel's lifecycle state.
	LifecycleState IpSecConnectionTunnelLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// The IP address of Oracle's VPN headend.
	// Example: `192.0.2.5`
	VpnIp *string `mandatory:"false" json:"vpnIp"`

	// The IP address of the CPE's VPN headend.
	// Example: `192.0.2.157`
	CpeIp *string `mandatory:"false" json:"cpeIp"`

	// The status of the tunnel based on IPSec protocol characteristics.
	Status IpSecConnectionTunnelStatusEnum `mandatory:"false" json:"status,omitempty"`

	// Internet Key Exchange protocol version.
	IkeVersion IpSecConnectionTunnelIkeVersionEnum `mandatory:"false" json:"ikeVersion,omitempty"`

	// A user-friendly name. Does not have to be unique, and it's changeable. Avoid
	// entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Information for establishing the tunnel's BGP session.
	BgpSessionInfo *BgpSessionInfo `mandatory:"false" json:"bgpSessionInfo"`

	// The type of routing used for this tunnel (either BGP dynamic routing or static routing).
	Routing IpSecConnectionTunnelRoutingEnum `mandatory:"false" json:"routing,omitempty"`

	// The date and time the IPSec connection tunnel was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`

	// When the status of the tunnel last changed, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeStatusUpdated *common.SDKTime `mandatory:"false" json:"timeStatusUpdated"`
}

func (m IpSecConnectionTunnel) String() string {
	return common.PointerString(m)
}

// IpSecConnectionTunnelStatusEnum Enum with underlying type: string
type IpSecConnectionTunnelStatusEnum string

// Set of constants representing the allowable values for IpSecConnectionTunnelStatusEnum
const (
	IpSecConnectionTunnelStatusUp                 IpSecConnectionTunnelStatusEnum = "UP"
	IpSecConnectionTunnelStatusDown               IpSecConnectionTunnelStatusEnum = "DOWN"
	IpSecConnectionTunnelStatusDownForMaintenance IpSecConnectionTunnelStatusEnum = "DOWN_FOR_MAINTENANCE"
)

var mappingIpSecConnectionTunnelStatus = map[string]IpSecConnectionTunnelStatusEnum{
	"UP":                   IpSecConnectionTunnelStatusUp,
	"DOWN":                 IpSecConnectionTunnelStatusDown,
	"DOWN_FOR_MAINTENANCE": IpSecConnectionTunnelStatusDownForMaintenance,
}

// GetIpSecConnectionTunnelStatusEnumValues Enumerates the set of values for IpSecConnectionTunnelStatusEnum
func GetIpSecConnectionTunnelStatusEnumValues() []IpSecConnectionTunnelStatusEnum {
	values := make([]IpSecConnectionTunnelStatusEnum, 0)
	for _, v := range mappingIpSecConnectionTunnelStatus {
		values = append(values, v)
	}
	return values
}

// IpSecConnectionTunnelIkeVersionEnum Enum with underlying type: string
type IpSecConnectionTunnelIkeVersionEnum string

// Set of constants representing the allowable values for IpSecConnectionTunnelIkeVersionEnum
const (
	IpSecConnectionTunnelIkeVersionV1 IpSecConnectionTunnelIkeVersionEnum = "V1"
	IpSecConnectionTunnelIkeVersionV2 IpSecConnectionTunnelIkeVersionEnum = "V2"
)

var mappingIpSecConnectionTunnelIkeVersion = map[string]IpSecConnectionTunnelIkeVersionEnum{
	"V1": IpSecConnectionTunnelIkeVersionV1,
	"V2": IpSecConnectionTunnelIkeVersionV2,
}

// GetIpSecConnectionTunnelIkeVersionEnumValues Enumerates the set of values for IpSecConnectionTunnelIkeVersionEnum
func GetIpSecConnectionTunnelIkeVersionEnumValues() []IpSecConnectionTunnelIkeVersionEnum {
	values := make([]IpSecConnectionTunnelIkeVersionEnum, 0)
	for _, v := range mappingIpSecConnectionTunnelIkeVersion {
		values = append(values, v)
	}
	return values
}

// IpSecConnectionTunnelLifecycleStateEnum Enum with underlying type: string
type IpSecConnectionTunnelLifecycleStateEnum string

// Set of constants representing the allowable values for IpSecConnectionTunnelLifecycleStateEnum
const (
	IpSecConnectionTunnelLifecycleStateProvisioning IpSecConnectionTunnelLifecycleStateEnum = "PROVISIONING"
	IpSecConnectionTunnelLifecycleStateAvailable    IpSecConnectionTunnelLifecycleStateEnum = "AVAILABLE"
	IpSecConnectionTunnelLifecycleStateTerminating  IpSecConnectionTunnelLifecycleStateEnum = "TERMINATING"
	IpSecConnectionTunnelLifecycleStateTerminated   IpSecConnectionTunnelLifecycleStateEnum = "TERMINATED"
)

var mappingIpSecConnectionTunnelLifecycleState = map[string]IpSecConnectionTunnelLifecycleStateEnum{
	"PROVISIONING": IpSecConnectionTunnelLifecycleStateProvisioning,
	"AVAILABLE":    IpSecConnectionTunnelLifecycleStateAvailable,
	"TERMINATING":  IpSecConnectionTunnelLifecycleStateTerminating,
	"TERMINATED":   IpSecConnectionTunnelLifecycleStateTerminated,
}

// GetIpSecConnectionTunnelLifecycleStateEnumValues Enumerates the set of values for IpSecConnectionTunnelLifecycleStateEnum
func GetIpSecConnectionTunnelLifecycleStateEnumValues() []IpSecConnectionTunnelLifecycleStateEnum {
	values := make([]IpSecConnectionTunnelLifecycleStateEnum, 0)
	for _, v := range mappingIpSecConnectionTunnelLifecycleState {
		values = append(values, v)
	}
	return values
}

// IpSecConnectionTunnelRoutingEnum Enum with underlying type: string
type IpSecConnectionTunnelRoutingEnum string

// Set of constants representing the allowable values for IpSecConnectionTunnelRoutingEnum
const (
	IpSecConnectionTunnelRoutingBgp    IpSecConnectionTunnelRoutingEnum = "BGP"
	IpSecConnectionTunnelRoutingStatic IpSecConnectionTunnelRoutingEnum = "STATIC"
)

var mappingIpSecConnectionTunnelRouting = map[string]IpSecConnectionTunnelRoutingEnum{
	"BGP":    IpSecConnectionTunnelRoutingBgp,
	"STATIC": IpSecConnectionTunnelRoutingStatic,
}

// GetIpSecConnectionTunnelRoutingEnumValues Enumerates the set of values for IpSecConnectionTunnelRoutingEnum
func GetIpSecConnectionTunnelRoutingEnumValues() []IpSecConnectionTunnelRoutingEnum {
	values := make([]IpSecConnectionTunnelRoutingEnum, 0)
	for _, v := range mappingIpSecConnectionTunnelRouting {
		values = append(values, v)
	}
	return values
}
