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

// Cpe An object you create when setting up an IPSec VPN between your on-premises network
// and VCN. The `Cpe` is a virtual representation of your customer-premises equipment,
// which is the actual router on-premises at your site at your end of the IPSec VPN connection.
// For more information,
// see Overview of the Networking Service (https://docs.cloud.oracle.com/Content/Network/Concepts/overview.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you're not authorized,
// talk to an administrator. If you're an administrator who needs to write policies to give users access, see
// Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
// **Warning:** Oracle recommends that you avoid using any confidential information when you
// supply string values using the API.
type Cpe struct {

	// The OCID of the compartment containing the CPE.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The CPE's Oracle ID (OCID).
	Id *string `mandatory:"true" json:"id"`

	// The public IP address of the on-premises router.
	IpAddress *string `mandatory:"true" json:"ipAddress"`

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

	// The OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of the CPE's device type.
	// The Networking service maintains a general list of CPE device types (for example,
	// Cisco ASA). For each type, Oracle provides CPE configuration content that can help
	// a network engineer configure the CPE. The OCID uniquely identifies the type of
	// device. To get the OCIDs for the device types on the list, see
	// ListCpeDeviceShapes.
	// For information about how to generate CPE configuration content for a
	// CPE device type, see:
	//   * GetCpeDeviceConfigContent
	//   * GetIpsecCpeDeviceConfigContent
	//   * GetTunnelCpeDeviceConfigContent
	//   * GetTunnelCpeDeviceConfig
	CpeDeviceShapeId *string `mandatory:"false" json:"cpeDeviceShapeId"`

	// The date and time the CPE was created, in the format defined by RFC3339.
	// Example: `2016-08-25T21:10:29.600Z`
	TimeCreated *common.SDKTime `mandatory:"false" json:"timeCreated"`
}

func (m Cpe) String() string {
	return common.PointerString(m)
}
