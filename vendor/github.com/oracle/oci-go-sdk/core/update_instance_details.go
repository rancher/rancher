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

// UpdateInstanceDetails The representation of UpdateInstanceDetails
type UpdateInstanceDetails struct {

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// A user-friendly name. Does not have to be unique, and it's changeable.
	// Avoid entering confidential information.
	// Example: `My bare metal instance`
	DisplayName *string `mandatory:"false" json:"displayName"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`

	// Instance agent configuration options to choose for updating the instance
	AgentConfig *UpdateInstanceAgentConfigDetails `mandatory:"false" json:"agentConfig"`

	// Custom metadata key/value string pairs that you provide. Any set of key/value pairs
	// provided here will completely replace the current set of key/value pairs in the 'metadata'
	// field on the instance.
	// Both the 'user_data' and 'ssh_authorized_keys' fields cannot be changed after an instance
	// has launched. Any request which updates, removes, or adds either of these fields will be
	// rejected. You must provide the same values for 'user_data' and 'ssh_authorized_keys' that
	// already exist on the instance.
	Metadata map[string]string `mandatory:"false" json:"metadata"`

	// Additional metadata key/value pairs that you provide. They serve the same purpose and
	// functionality as fields in the 'metadata' object.
	// They are distinguished from 'metadata' fields in that these can be nested JSON objects
	// (whereas 'metadata' fields are string/string maps only).
	// Both the 'user_data' and 'ssh_authorized_keys' fields cannot be changed after an instance
	// has launched. Any request which updates, removes, or adds either of these fields will be
	// rejected. You must provide the same values for 'user_data' and 'ssh_authorized_keys' that
	// already exist on the instance.
	ExtendedMetadata map[string]interface{} `mandatory:"false" json:"extendedMetadata"`

	// The shape of the instance. The shape determines the number of CPUs and the amount of memory
	// allocated to the instance. For more information about how to change shapes, and a list of
	// shapes that are supported, see
	// Changing the Shape of an Instance (https://docs.cloud.oracle.com/iaas/Content/Compute/Tasks/resizinginstances.htm).
	// For details about the CPUs, memory, and other properties of each shape, see
	// Compute Shapes (https://docs.cloud.oracle.com/iaas/Content/Compute/References/computeshapes.htm).
	// The new shape must be compatible with the image that was used to launch the instance. You
	// can enumerate all available shapes and determine image compatibility by calling
	// ListShapes.
	// If the instance is running when you change the shape, the instance is rebooted.
	// Example: `VM.Standard2.1`
	Shape *string `mandatory:"false" json:"shape"`

	ShapeConfig *UpdateInstanceShapeConfigDetails `mandatory:"false" json:"shapeConfig"`
}

func (m UpdateInstanceDetails) String() string {
	return common.PointerString(m)
}
