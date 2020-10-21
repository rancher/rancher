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

// VolumeBackupPolicyAssignment Specifies the volume that the volume backup policy is assigned to.
// For more information about Oracle defined backup policies and custom backup policies,
// see Policy-Based Backups (https://docs.cloud.oracle.com/iaas/Content/Block/Tasks/schedulingvolumebackups.htm).
type VolumeBackupPolicyAssignment struct {

	// The OCID of the volume the policy has been assigned to.
	AssetId *string `mandatory:"true" json:"assetId"`

	// The OCID of the volume backup policy assignment.
	Id *string `mandatory:"true" json:"id"`

	// The OCID of the volume backup policy that has been assigned to the volume.
	PolicyId *string `mandatory:"true" json:"policyId"`

	// The date and time the volume backup policy was assigned to the volume. The format is defined by RFC3339.
	TimeCreated *common.SDKTime `mandatory:"true" json:"timeCreated"`
}

func (m VolumeBackupPolicyAssignment) String() string {
	return common.PointerString(m)
}
