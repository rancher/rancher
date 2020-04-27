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

// RemotePeeringConnection A remote peering connection (RPC) is an object on a DRG that lets the VCN that is attached
// to the DRG peer with a VCN in a different region. *Peering* means that the two VCNs can
// communicate using private IP addresses, but without the traffic traversing the internet or
// routing through your on-premises network. For more information, see
// VCN Peering (https://docs.cloud.oracle.com/Content/Network/Tasks/VCNpeering.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you're not authorized,
// talk to an administrator. If you're an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
// **Warning:** Oracle recommends that you avoid using any confidential information when you
// supply string values using the API.
type RemotePeeringConnection struct {

	// The OCID of the compartment that contains the RPC.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"true" json:"displayName"`

	// The OCID of the DRG that this RPC belongs to.
	DrgId *string `mandatory:"true" json:"drgId"`

	// The OCID of the RPC.
	Id *string `mandatory:"true" json:"id"`

	// Whether the VCN at the other end of the peering is in a different tenancy.
	// Example: `false`
	IsCrossTenancyPeering *bool `mandatory:"true" json:"isCrossTenancyPeering"`

	// The RPC's current lifecycle state.
	LifecycleState RemotePeeringConnectionLifecycleStateEnum `mandatory:"true" json:"lifecycleState"`

	// Whether the RPC is peered with another RPC. `NEW` means the RPC has not yet been
	// peered. `PENDING` means the peering is being established. `REVOKED` means the
	// RPC at the other end of the peering has been deleted.
	PeeringStatus RemotePeeringConnectionPeeringStatusEnum `mandatory:"true" json:"peeringStatus"`

	// The date and time the RPC was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// If this RPC is peered, this value is the OCID of the other RPC.
	PeerId *string `mandatory:"false" json:"peerId"`

	// If this RPC is peered, this value is the region that contains the other RPC.
	// Example: `us-ashburn-1`
	PeerRegionName *string `mandatory:"false" json:"peerRegionName"`

	// If this RPC is peered, this value is the OCID of the other RPC's tenancy.
	PeerTenancyId *string `mandatory:"false" json:"peerTenancyId"`
}

func (m RemotePeeringConnection) String() string {
	return common.PointerString(m)
}

// RemotePeeringConnectionLifecycleStateEnum Enum with underlying type: string
type RemotePeeringConnectionLifecycleStateEnum string

// Set of constants representing the allowable values for RemotePeeringConnectionLifecycleStateEnum
const (
	RemotePeeringConnectionLifecycleStateAvailable    RemotePeeringConnectionLifecycleStateEnum = "AVAILABLE"
	RemotePeeringConnectionLifecycleStateProvisioning RemotePeeringConnectionLifecycleStateEnum = "PROVISIONING"
	RemotePeeringConnectionLifecycleStateTerminating  RemotePeeringConnectionLifecycleStateEnum = "TERMINATING"
	RemotePeeringConnectionLifecycleStateTerminated   RemotePeeringConnectionLifecycleStateEnum = "TERMINATED"
)

var mappingRemotePeeringConnectionLifecycleState = map[string]RemotePeeringConnectionLifecycleStateEnum{
	"AVAILABLE":    RemotePeeringConnectionLifecycleStateAvailable,
	"PROVISIONING": RemotePeeringConnectionLifecycleStateProvisioning,
	"TERMINATING":  RemotePeeringConnectionLifecycleStateTerminating,
	"TERMINATED":   RemotePeeringConnectionLifecycleStateTerminated,
}

// GetRemotePeeringConnectionLifecycleStateEnumValues Enumerates the set of values for RemotePeeringConnectionLifecycleStateEnum
func GetRemotePeeringConnectionLifecycleStateEnumValues() []RemotePeeringConnectionLifecycleStateEnum {
	values := make([]RemotePeeringConnectionLifecycleStateEnum, 0)
	for _, v := range mappingRemotePeeringConnectionLifecycleState {
		values = append(values, v)
	}
	return values
}

// RemotePeeringConnectionPeeringStatusEnum Enum with underlying type: string
type RemotePeeringConnectionPeeringStatusEnum string

// Set of constants representing the allowable values for RemotePeeringConnectionPeeringStatusEnum
const (
	RemotePeeringConnectionPeeringStatusInvalid RemotePeeringConnectionPeeringStatusEnum = "INVALID"
	RemotePeeringConnectionPeeringStatusNew     RemotePeeringConnectionPeeringStatusEnum = "NEW"
	RemotePeeringConnectionPeeringStatusPending RemotePeeringConnectionPeeringStatusEnum = "PENDING"
	RemotePeeringConnectionPeeringStatusPeered  RemotePeeringConnectionPeeringStatusEnum = "PEERED"
	RemotePeeringConnectionPeeringStatusRevoked RemotePeeringConnectionPeeringStatusEnum = "REVOKED"
)

var mappingRemotePeeringConnectionPeeringStatus = map[string]RemotePeeringConnectionPeeringStatusEnum{
	"INVALID": RemotePeeringConnectionPeeringStatusInvalid,
	"NEW":     RemotePeeringConnectionPeeringStatusNew,
	"PENDING": RemotePeeringConnectionPeeringStatusPending,
	"PEERED":  RemotePeeringConnectionPeeringStatusPeered,
	"REVOKED": RemotePeeringConnectionPeeringStatusRevoked,
}

// GetRemotePeeringConnectionPeeringStatusEnumValues Enumerates the set of values for RemotePeeringConnectionPeeringStatusEnum
func GetRemotePeeringConnectionPeeringStatusEnumValues() []RemotePeeringConnectionPeeringStatusEnum {
	values := make([]RemotePeeringConnectionPeeringStatusEnum, 0)
	for _, v := range mappingRemotePeeringConnectionPeeringStatus {
		values = append(values, v)
	}
	return values
}
