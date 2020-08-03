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

// InstanceConfigurationVolumeSourceDetails The representation of InstanceConfigurationVolumeSourceDetails
type InstanceConfigurationVolumeSourceDetails interface {
}

type instanceconfigurationvolumesourcedetails struct {
	JsonData []byte
	Type     string `json:"type"`
}

// UnmarshalJSON unmarshals json
func (m *instanceconfigurationvolumesourcedetails) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalerinstanceconfigurationvolumesourcedetails instanceconfigurationvolumesourcedetails
	s := struct {
		Model Unmarshalerinstanceconfigurationvolumesourcedetails
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.Type = s.Model.Type

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *instanceconfigurationvolumesourcedetails) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.Type {
	case "volumeBackup":
		mm := InstanceConfigurationVolumeSourceFromVolumeBackupDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	case "volume":
		mm := InstanceConfigurationVolumeSourceFromVolumeDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		return *m, nil
	}
}

func (m instanceconfigurationvolumesourcedetails) String() string {
	return common.PointerString(m)
}
