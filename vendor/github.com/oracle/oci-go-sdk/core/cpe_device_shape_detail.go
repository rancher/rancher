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

// CpeDeviceShapeDetail The detailed information about a particular CPE device type. Compare with
// CpeDeviceShapeSummary.
type CpeDeviceShapeDetail struct {

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the CPE device shape.
	// This value uniquely identifies the type of CPE device.
	CpeDeviceShapeId *string `mandatory:"false" json:"cpeDeviceShapeId"`

	// Basic information about this particular CPE device type.
	CpeDeviceInfo *CpeDeviceInfo `mandatory:"false" json:"cpeDeviceInfo"`

	// For certain CPE devices types, the customer can provide answers to
	// questions that are specific to the device type. This attribute contains
	// a list of those questions. The Networking service merges the answers with
	// other information and renders a set of CPE configuration content. To
	// provide the answers, use
	// UpdateTunnelCpeDeviceConfig.
	Parameters []CpeDeviceConfigQuestion `mandatory:"false" json:"parameters"`

	// A template of CPE device configuration information that will be merged with the customer's
	// answers to the questions to render the final CPE device configuration content. Also see:
	//   * GetCpeDeviceConfigContent
	//   * GetIpsecCpeDeviceConfigContent
	//   * GetTunnelCpeDeviceConfigContent
	Template *string `mandatory:"false" json:"template"`
}

func (m CpeDeviceShapeDetail) String() string {
	return common.PointerString(m)
}
