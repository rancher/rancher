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

// CreateCrossConnectDetails The representation of CreateCrossConnectDetails
type CreateCrossConnectDetails struct {

	// The OCID of the compartment to contain the cross-connect.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The name of the FastConnect location where this cross-connect will be installed.
	// To get a list of the available locations, see
	// ListCrossConnectLocations.
	// Example: `CyrusOne, Chandler, AZ`
	LocationName *string `mandatory:"true" json:"locationName"`

	// The port speed for this cross-connect. To get a list of the available port speeds, see
	// ListCrossconnectPortSpeedShapes.
	// Example: `10 Gbps`
	PortSpeedShapeName *string `mandatory:"true" json:"portSpeedShapeName"`

	// The OCID of the cross-connect group to put this cross-connect in.
	CrossConnectGroupId *string `mandatory:"false" json:"crossConnectGroupId"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// If you already have an existing cross-connect or cross-connect group at this FastConnect
	// location, and you want this new cross-connect to be on a different router (for the
	// purposes of redundancy), provide the OCID of that existing cross-connect or
	// cross-connect group.
	FarCrossConnectOrCrossConnectGroupId *string `mandatory:"false" json:"farCrossConnectOrCrossConnectGroupId"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// If you already have an existing cross-connect or cross-connect group at this FastConnect
	// location, and you want this new cross-connect to be on the same router, provide the
	// OCID of that existing cross-connect or cross-connect group.
	NearCrossConnectOrCrossConnectGroupId *string `mandatory:"false" json:"nearCrossConnectOrCrossConnectGroupId"`

	// A reference name or identifier for the physical fiber connection that this cross-connect
	// uses.
	CustomerReferenceName *string `mandatory:"false" json:"customerReferenceName"`
}

func (m CreateCrossConnectDetails) String() string {
	return common.PointerString(m)
}
