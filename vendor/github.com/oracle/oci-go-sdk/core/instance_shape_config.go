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

// InstanceShapeConfig The shape configuration for an instance. The shape configuration determines
// the resources allocated to an instance.
type InstanceShapeConfig struct {

	// The total number of OCPUs available to the instance.
	Ocpus *float32 `mandatory:"false" json:"ocpus"`

	// The total amount of memory, in gigabytes, available to the instance.
	MemoryInGBs *float32 `mandatory:"false" json:"memoryInGBs"`

	// A short description of the processors available to the instance.
	ProcessorDescription *string `mandatory:"false" json:"processorDescription"`

	// The networking bandwidth, in gigabits per second, available to the instance.
	NetworkingBandwidthInGbps *float32 `mandatory:"false" json:"networkingBandwidthInGbps"`

	// The maximum number of VNIC attachments for the instance.
	MaxVnicAttachments *int `mandatory:"false" json:"maxVnicAttachments"`

	// The number of GPUs available to this instance.
	Gpus *int `mandatory:"false" json:"gpus"`

	// A short description of the GPUs available to this instance.
	// This field is `null` if `gpus` is `0`.
	GpuDescription *string `mandatory:"false" json:"gpuDescription"`

	// The number of local disks available to the instance.
	LocalDisks *int `mandatory:"false" json:"localDisks"`

	// The size of the local disks, aggregated, in gigabytes.
	// This field is `null` if `localDisks` is equal to `0`.
	LocalDisksTotalSizeInGBs *float32 `mandatory:"false" json:"localDisksTotalSizeInGBs"`

	// A short description of the local disks available to this instance.
	// This field is `null` if `localDisks` is equal to `0`.
	LocalDiskDescription *string `mandatory:"false" json:"localDiskDescription"`
}

func (m InstanceShapeConfig) String() string {
	return common.PointerString(m)
}
