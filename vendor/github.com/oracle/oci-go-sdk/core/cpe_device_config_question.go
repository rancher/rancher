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

// CpeDeviceConfigQuestion An individual question that the customer can answer about the CPE device.
// The customer provides answers to these questions in
// UpdateTunnelCpeDeviceConfig.
type CpeDeviceConfigQuestion struct {

	// A string that identifies the question.
	Key *string `mandatory:"false" json:"key"`

	// A descriptive label for the question (for example, to display in a form in a graphical interface).
	DisplayName *string `mandatory:"false" json:"displayName"`

	// A description or explanation of the question, to help the customer answer accurately.
	Explanation *string `mandatory:"false" json:"explanation"`
}

func (m CpeDeviceConfigQuestion) String() string {
	return common.PointerString(m)
}
