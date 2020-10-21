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

// CrossConnect For use with Oracle Cloud Infrastructure FastConnect. A cross-connect represents a
// physical connection between an existing network and Oracle. Customers who are colocated
// with Oracle in a FastConnect location create and use cross-connects. For more
// information, see FastConnect Overview (https://docs.cloud.oracle.com/Content/Network/Concepts/fastconnect.htm).
// Oracle recommends you create each cross-connect in a
// CrossConnectGroup so you can use link aggregation
// with the connection.
// **Note:** If you're a provider who is setting up a physical connection to Oracle so customers
// can use FastConnect over the connection, be aware that your connection is modeled the
// same way as a colocated customer's (with `CrossConnect` and `CrossConnectGroup` objects, and so on).
// To use any of the API operations, you must be authorized in an IAM policy. If you're not authorized,
// talk to an administrator. If you're an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
// **Warning:** Oracle recommends that you avoid using any confidential information when you
// supply string values using the API.
type CrossConnect struct {

	// The OCID of the compartment containing the cross-connect group.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// The OCID of the cross-connect group this cross-connect belongs to (if any).
	CrossConnectGroupId *string `mandatory:"false" json:"crossConnectGroupId"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// The cross-connect's Oracle ID (OCID).
	Id *string `mandatory:"false" json:"id"`

	// The cross-connect's current state.
	LifecycleState CrossConnectLifecycleStateEnum `mandatory:"false" json:"lifecycleState,omitempty"`

	// The name of the FastConnect location where this cross-connect is installed.
	LocationName *string `mandatory:"false" json:"locationName"`

	// A string identifying the meet-me room port for this cross-connect.
	PortName *string `mandatory:"false" json:"portName"`

	// The port speed for this cross-connect.
	// Example: `10 Gbps`
	PortSpeedShapeName *string `mandatory:"false" json:"portSpeedShapeName"`

	// A reference name or identifier for the physical fiber connection that this cross-connect
	// uses.
	CustomerReferenceName *string `mandatory:"false" json:"customerReferenceName"`

	// The date and time the cross-connect was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`
}

func (m CrossConnect) String() string {
	return common.PointerString(m)
}

// CrossConnectLifecycleStateEnum Enum with underlying type: string
type CrossConnectLifecycleStateEnum string

// Set of constants representing the allowable values for CrossConnectLifecycleStateEnum
const (
	CrossConnectLifecycleStatePendingCustomer CrossConnectLifecycleStateEnum = "PENDING_CUSTOMER"
	CrossConnectLifecycleStateProvisioning    CrossConnectLifecycleStateEnum = "PROVISIONING"
	CrossConnectLifecycleStateProvisioned     CrossConnectLifecycleStateEnum = "PROVISIONED"
	CrossConnectLifecycleStateInactive        CrossConnectLifecycleStateEnum = "INACTIVE"
	CrossConnectLifecycleStateTerminating     CrossConnectLifecycleStateEnum = "TERMINATING"
	CrossConnectLifecycleStateTerminated      CrossConnectLifecycleStateEnum = "TERMINATED"
)

var mappingCrossConnectLifecycleState = map[string]CrossConnectLifecycleStateEnum{
	"PENDING_CUSTOMER": CrossConnectLifecycleStatePendingCustomer,
	"PROVISIONING":     CrossConnectLifecycleStateProvisioning,
	"PROVISIONED":      CrossConnectLifecycleStateProvisioned,
	"INACTIVE":         CrossConnectLifecycleStateInactive,
	"TERMINATING":      CrossConnectLifecycleStateTerminating,
	"TERMINATED":       CrossConnectLifecycleStateTerminated,
}

// GetCrossConnectLifecycleStateEnumValues Enumerates the set of values for CrossConnectLifecycleStateEnum
func GetCrossConnectLifecycleStateEnumValues() []CrossConnectLifecycleStateEnum {
	values := make([]CrossConnectLifecycleStateEnum, 0)
	for _, v := range mappingCrossConnectLifecycleState {
		values = append(values, v)
	}
	return values
}
