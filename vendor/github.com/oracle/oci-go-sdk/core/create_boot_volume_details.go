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
	"encoding/json"
	"github.com/oracle/oci-go-sdk/common"
)

// CreateBootVolumeDetails The representation of CreateBootVolumeDetails
type CreateBootVolumeDetails struct {

	// The availability domain of the boot volume.
	// Example: `Uocm:PHX-AD-1`
	AvailabilityDomain *string `mandatory:"true" json:"availabilityDomain"`

	// The OCID of the compartment that contains the boot volume.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// Specifies the boot volume source details for a new boot volume. The volume source is either another boot volume in the same availability domain or a boot volume backup.
	// This is a mandatory field for a boot volume.
	SourceDetails BootVolumeSourceDetails `mandatory:"true" json:"sourceDetails"`

	// If provided, specifies the ID of the boot volume backup policy to assign to the newly
	// created boot volume. If omitted, no policy will be assigned.
	BackupPolicyId *string `mandatory:"false" json:"backupPolicyId"`

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

	// The OCID of the Key Management key to assign as the master encryption key
	// for the boot volume.
	KmsKeyId *string `mandatory:"false" json:"kmsKeyId"`

	// The size of the volume in GBs.
	SizeInGBs *int64 `mandatory:"false" json:"sizeInGBs"`

	// The number of volume performance units (VPUs) that will be applied to this volume per GB,
	// representing the Block Volume service's elastic performance options.
	// See Block Volume Elastic Performance (https://docs.cloud.oracle.com/Content/Block/Concepts/blockvolumeelasticperformance.htm) for more information.
	// Allowed values:
	//   * `10`: Represents Balanced option.
	//   * `20`: Represents Higher Performance option.
	VpusPerGB *int64 `mandatory:"false" json:"vpusPerGB"`
}

func (m CreateBootVolumeDetails) String() string {
	return common.PointerString(m)
}

// UnmarshalJSON unmarshals from json
func (m *CreateBootVolumeDetails) UnmarshalJSON(data []byte) (e error) {
	model := struct {
		BackupPolicyId     *string                           `json:"backupPolicyId"`
		DefinedTags        map[string]map[string]interface{} `json:"definedTags"`
		DisplayName        *string                           `json:"displayName"`
		FreeformTags       map[string]string                 `json:"freeformTags"`
		KmsKeyId           *string                           `json:"kmsKeyId"`
		SizeInGBs          *int64                            `json:"sizeInGBs"`
		VpusPerGB          *int64                            `json:"vpusPerGB"`
		AvailabilityDomain *string                           `json:"availabilityDomain"`
		CompartmentId      *string                           `json:"compartmentId"`
		SourceDetails      bootvolumesourcedetails           `json:"sourceDetails"`
	}{}

	e = json.Unmarshal(data, &model)
	if e != nil {
		return
	}
	var nn interface{}
	m.BackupPolicyId = model.BackupPolicyId

	m.DefinedTags = model.DefinedTags

	m.DisplayName = model.DisplayName

	m.FreeformTags = model.FreeformTags

	m.KmsKeyId = model.KmsKeyId

	m.SizeInGBs = model.SizeInGBs

	m.VpusPerGB = model.VpusPerGB

	m.AvailabilityDomain = model.AvailabilityDomain

	m.CompartmentId = model.CompartmentId

	nn, e = model.SourceDetails.UnmarshalPolymorphicJSON(model.SourceDetails.JsonData)
	if e != nil {
		return
	}
	if nn != nil {
		m.SourceDetails = nn.(BootVolumeSourceDetails)
	} else {
		m.SourceDetails = nil
	}
	return
}
