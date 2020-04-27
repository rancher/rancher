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

// ShapeMaxVnicAttachmentOptions The possible configurations for the number of VNIC attachments available to an instance of this shape. If this field is null, then all instances of this shape have a fixed maximum number of VNIC attachments equal to `maxVnicAttachments`.
type ShapeMaxVnicAttachmentOptions struct {

	// The lowest maximum value of VNIC attachments.
	Min *int `mandatory:"false" json:"min"`

	// The highest maximum value of VNIC attachments.
	Max *float32 `mandatory:"false" json:"max"`

	// The default number of VNIC attachments allowed per OCPU.
	DefaultPerOcpu *float32 `mandatory:"false" json:"defaultPerOcpu"`
}

func (m ShapeMaxVnicAttachmentOptions) String() string {
	return common.PointerString(m)
}
