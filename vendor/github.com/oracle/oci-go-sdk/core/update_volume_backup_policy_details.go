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

// UpdateVolumeBackupPolicyDetails Specifies the properties for a updating a user defined backup policy.
// For more information about user defined backup policies,
// see User Defined Policies (https://docs.cloud.oracle.com/iaas/Content/Block/Tasks/schedulingvolumebackups.htm#UserDefinedBackupPolicies) in
// Policy-Based Backups (https://docs.cloud.oracle.com/iaas/Content/Block/Tasks/schedulingvolumebackups.htm).
type UpdateVolumeBackupPolicyDetails struct {

	// A user-friendly name for the volume backup policy. Does not have to be unique and it's changeable.
	// Avoid entering confidential information.
	DisplayName *string `mandatory:"false" json:"displayName"`

	// The paired destination region (pre-defined by oracle) for scheduled cross region backup calls. Example: `us-ashburn-1`
	DestinationRegion *string `mandatory:"false" json:"destinationRegion"`

	// The collection of schedules for the volume backup policy. See
	// see Schedules (https://docs.cloud.oracle.com/iaas/Content/Block/Tasks/schedulingvolumebackups.htm#schedules) in
	// Policy-Based Backups (https://docs.cloud.oracle.com/iaas/Content/Block/Tasks/schedulingvolumebackups.htm) for more information.
	Schedules []VolumeBackupSchedule `mandatory:"false" json:"schedules"`

	// Defined tags for this resource. Each key is predefined and scoped to a
	// namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Operations": {"CostCenter": "42"}}`
	DefinedTags map[string]map[string]interface{} `mandatory:"false" json:"definedTags"`

	// Free-form tags for this resource. Each tag is a simple key-value pair with no
	// predefined name, type, or namespace. For more information, see Resource Tags (https://docs.cloud.oracle.com/Content/General/Concepts/resourcetags.htm).
	// Example: `{"Department": "Finance"}`
	FreeformTags map[string]string `mandatory:"false" json:"freeformTags"`
}

func (m UpdateVolumeBackupPolicyDetails) String() string {
	return common.PointerString(m)
}
