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

// DrgRedundancyStatus The redundancy status of the DRG. For more information, see
// Redundancy Remedies (https://docs.cloud.oracle.com/Content/Network/Troubleshoot/drgredundancy.htm).
type DrgRedundancyStatus struct {

	// The OCID of the DRG.
	Id *string `mandatory:"false" json:"id"`

	// The redundancy status of the DRG.
	Status DrgRedundancyStatusStatusEnum `mandatory:"false" json:"status,omitempty"`
}

func (m DrgRedundancyStatus) String() string {
	return common.PointerString(m)
}

// DrgRedundancyStatusStatusEnum Enum with underlying type: string
type DrgRedundancyStatusStatusEnum string

// Set of constants representing the allowable values for DrgRedundancyStatusStatusEnum
const (
	DrgRedundancyStatusStatusNotAvailable                        DrgRedundancyStatusStatusEnum = "NOT_AVAILABLE"
	DrgRedundancyStatusStatusRedundant                           DrgRedundancyStatusStatusEnum = "REDUNDANT"
	DrgRedundancyStatusStatusNotRedundantSingleIpsec             DrgRedundancyStatusStatusEnum = "NOT_REDUNDANT_SINGLE_IPSEC"
	DrgRedundancyStatusStatusNotRedundantSingleVirtualcircuit    DrgRedundancyStatusStatusEnum = "NOT_REDUNDANT_SINGLE_VIRTUALCIRCUIT"
	DrgRedundancyStatusStatusNotRedundantMultipleIpsecs          DrgRedundancyStatusStatusEnum = "NOT_REDUNDANT_MULTIPLE_IPSECS"
	DrgRedundancyStatusStatusNotRedundantMultipleVirtualcircuits DrgRedundancyStatusStatusEnum = "NOT_REDUNDANT_MULTIPLE_VIRTUALCIRCUITS"
	DrgRedundancyStatusStatusNotRedundantMixConnections          DrgRedundancyStatusStatusEnum = "NOT_REDUNDANT_MIX_CONNECTIONS"
	DrgRedundancyStatusStatusNotRedundantNoConnection            DrgRedundancyStatusStatusEnum = "NOT_REDUNDANT_NO_CONNECTION"
)

var mappingDrgRedundancyStatusStatus = map[string]DrgRedundancyStatusStatusEnum{
	"NOT_AVAILABLE":                          DrgRedundancyStatusStatusNotAvailable,
	"REDUNDANT":                              DrgRedundancyStatusStatusRedundant,
	"NOT_REDUNDANT_SINGLE_IPSEC":             DrgRedundancyStatusStatusNotRedundantSingleIpsec,
	"NOT_REDUNDANT_SINGLE_VIRTUALCIRCUIT":    DrgRedundancyStatusStatusNotRedundantSingleVirtualcircuit,
	"NOT_REDUNDANT_MULTIPLE_IPSECS":          DrgRedundancyStatusStatusNotRedundantMultipleIpsecs,
	"NOT_REDUNDANT_MULTIPLE_VIRTUALCIRCUITS": DrgRedundancyStatusStatusNotRedundantMultipleVirtualcircuits,
	"NOT_REDUNDANT_MIX_CONNECTIONS":          DrgRedundancyStatusStatusNotRedundantMixConnections,
	"NOT_REDUNDANT_NO_CONNECTION":            DrgRedundancyStatusStatusNotRedundantNoConnection,
}

// GetDrgRedundancyStatusStatusEnumValues Enumerates the set of values for DrgRedundancyStatusStatusEnum
func GetDrgRedundancyStatusStatusEnumValues() []DrgRedundancyStatusStatusEnum {
	values := make([]DrgRedundancyStatusStatusEnum, 0)
	for _, v := range mappingDrgRedundancyStatusStatus {
		values = append(values, v)
	}
	return values
}
