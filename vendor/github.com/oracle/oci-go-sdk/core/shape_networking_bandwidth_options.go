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

// ShapeNetworkingBandwidthOptions The possible configurations for the amount of networking bandwidth available to an instance of this shape. If this field is null, then all instances of this shape have a fixed amount of bandwidth equivalent to `networkingBandwidthInGbps`.
type ShapeNetworkingBandwidthOptions struct {

	// The minimum amount of networking bandwidth, in gigabits per second.
	MinInGbps *float32 `mandatory:"false" json:"minInGbps"`

	// The maximum amount of networking bandwidth, in gigabits per second.
	MaxInGbps *float32 `mandatory:"false" json:"maxInGbps"`

	// The default amount of networking bandwidth, in gigabits per second,
	// per OCPU.
	DefaultPerOcpuInGbps *float32 `mandatory:"false" json:"defaultPerOcpuInGbps"`
}

func (m ShapeNetworkingBandwidthOptions) String() string {
	return common.PointerString(m)
}
