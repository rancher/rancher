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

// Vcn A virtual cloud network (VCN). For more information, see
// Overview of the Networking Service (https://docs.cloud.oracle.com/Content/Network/Concepts/overview.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you're not authorized,
// talk to an administrator. If you're an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
// **Warning:** Oracle recommends that you avoid using any confidential information when you
// supply string values using the API.
type Vcn struct {

	// The CIDR IP address block of the VCN.
	// Example: `172.16.0.0/16`
	CidrBlock *string `mandatory:"true" json:"cidrBlock"`

	// The OCID of the compartment containing the VCN.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The VCN's Oracle ID (OCID).
	Id *string `mandatory:"true" json:"id"`

	// The VCN's current state.
	LifecycleState VcnLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// The OCID for the VCN's default set of DHCP options.
	DefaultDhcpOptionsId *string `mandatory:"false" json:"defaultDhcpOptionsId"`

	// The OCID for the VCN's default route table.
	DefaultRouteTableId *string `mandatory:"false" json:"defaultRouteTableId"`

	// The OCID for the VCN's default security list.
	DefaultSecurityListId *string `mandatory:"false" json:"defaultSecurityListId"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// A DNS label for the VCN, used in conjunction with the VNIC's hostname and
	// subnet's DNS label to form a fully qualified domain name (FQDN) for each VNIC
	// within this subnet (for example, `bminstance-1.subnet123.vcn1.oraclevcn.com`).
	// Must be an alphanumeric string that begins with a letter.
	// The value cannot be changed.
	// The absence of this parameter means the Internet and VCN Resolver will
	// not work for this VCN.
	// For more information, see
	// DNS in Your Virtual Cloud Network (https://docs.cloud.oracle.com/Content/Network/Concepts/dns.htm).
	// Example: `vcn1`
	DnsLabel *string `mandatory:"false" json:"dnsLabel"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// For an IPv6-enabled VCN, this is the IPv6 CIDR block for the VCN's private IP address space.
	// The VCN size is always /48. If you don't provide a value when creating the VCN, Oracle
	// provides one and uses that *same* CIDR for the `ipv6PublicCidrBlock`. If you do provide a
	// value, Oracle provides a *different* CIDR for the `ipv6PublicCidrBlock`. Note that IPv6
	// addressing is currently supported only in certain regions. See
	// IPv6 Addresses (https://docs.cloud.oracle.com/Content/Network/Concepts/ipv6.htm).
	// Example: `2001:0db8:0123::/48`
	Ipv6CidrBlock *string `mandatory:"false" json:"ipv6CidrBlock"`

	// For an IPv6-enabled VCN, this is the IPv6 CIDR block for the VCN's public IP address space.
	// The VCN size is always /48. This CIDR is always provided by Oracle. If you don't provide a
	// custom CIDR for the `ipv6CidrBlock` when creating the VCN, Oracle assigns that value and also
	// uses it for `ipv6PublicCidrBlock`. Oracle uses addresses from this block for the `publicIpAddress`
	// attribute of an Ipv6 that has internet access allowed.
	// Example: `2001:0db8:0123::/48`
	Ipv6PublicCidrBlock *string `mandatory:"false" json:"ipv6PublicCidrBlock"`

	// The date and time the VCN was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`

	// The VCN's domain name, which consists of the VCN's DNS label, and the
	// `oraclevcn.com` domain.
	// For more information, see
	// DNS in Your Virtual Cloud Network (https://docs.cloud.oracle.com/Content/Network/Concepts/dns.htm).
	// Example: `vcn1.oraclevcn.com`
	VcnDomainName *string `mandatory:"false" json:"vcnDomainName"`
}

func (m Vcn) String() string {
	return common.PointerString(m)
}

// VcnLifecycleStateEnum Enum with underlying type: string
type VcnLifecycleStateEnum string

// Set of constants representing the allowable values for VcnLifecycleStateEnum
const (
	VcnLifecycleStateProvisioning VcnLifecycleStateEnum = "PROVISIONING"
	VcnLifecycleStateAvailable    VcnLifecycleStateEnum = "AVAILABLE"
	VcnLifecycleStateTerminating  VcnLifecycleStateEnum = "TERMINATING"
	VcnLifecycleStateTerminated   VcnLifecycleStateEnum = "TERMINATED"
)

var mappingVcnLifecycleState = map[string]VcnLifecycleStateEnum{
	"PROVISIONING": VcnLifecycleStateProvisioning,
	"AVAILABLE":    VcnLifecycleStateAvailable,
	"TERMINATING":  VcnLifecycleStateTerminating,
	"TERMINATED":   VcnLifecycleStateTerminated,
}

// GetVcnLifecycleStateEnumValues Enumerates the set of values for VcnLifecycleStateEnum
func GetVcnLifecycleStateEnumValues() []VcnLifecycleStateEnum {
	values := make([]VcnLifecycleStateEnum, 0)
	for _, v := range mappingVcnLifecycleState {
		values = append(values, v)
	}
	return values
}
