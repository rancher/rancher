// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Identity and Access Management Service API
//
// APIs for managing users, groups, compartments, and policies.
//

package identity

import (
	"github.com/oracle/oci-go-sdk/common"
)

// Region A localized geographic area, such as Phoenix, AZ. Oracle Cloud Infrastructure is hosted in regions and Availability
// Domains. A region is composed of several Availability Domains. An Availability Domain is one or more data centers
// located within a region. For more information, see Regions and Availability Domains (https://docs.cloud.oracle.com/Content/General/Concepts/regions.htm).
// To use any of the API operations, you must be authorized in an IAM policy. If you're not authorized,
// talk to an administrator. If you're an administrator who needs to write policies to give users access,
// see Getting Started with Policies (https://docs.cloud.oracle.com/Content/Identity/Concepts/policygetstarted.htm).
type Region struct {

	// The key of the region.
	// Allowed values are:
	// - `PHX`
	// - `IAD`
	// - `FRA`
	// - `LHR`
	// - `YYZ`
	// - `NRT`
	// - `ICN`
	Key *string `mandatory:"false" json:"key"`

	// The name of the region.
	// Allowed values are:
	// - `ap-seoul-1`
	// - `ap-tokyo-1`
	// - `ca-toronto-1`
	// - `eu-frankurt-1`
	// - `uk-london-1`
	// - `us-ashburn-1`
	// - `us-phoenix-1`
	Name *string `mandatory:"false" json:"name"`
}

func (m Region) String() string {
	return common.PointerString(m)
}
