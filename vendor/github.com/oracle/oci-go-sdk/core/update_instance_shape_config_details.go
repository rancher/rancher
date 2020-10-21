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

// UpdateInstanceShapeConfigDetails The shape configuration requested for the instance. If provided, the instance will be updated
// with the resources specified. In the case where some properties are missing,
// the missing values will be set to the default for the provided `shape`.
// Each shape only supports certain configurable values. If the `shape` is provided
// and the configuration values are invalid for that new `shape`, an error will be returned.
// If no `shape` is provided and the configuration values are invalid for the instance's
// existing shape, an error will be returned.
type UpdateInstanceShapeConfigDetails struct {

	// The total number of OCPUs available to the instance.
	Ocpus *float32 `mandatory:"false" json:"ocpus"`
}

func (m UpdateInstanceShapeConfigDetails) String() string {
	return common.PointerString(m)
}
