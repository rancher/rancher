// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Container Engine for Kubernetes API
//
// API for the Container Engine for Kubernetes service. Use this API to build, deploy,
// and manage cloud-native applications. For more information, see
// Overview of Container Engine for Kubernetes (https://docs.cloud.oracle.com/iaas/Content/ContEng/Concepts/contengoverview.htm).
//

package containerengine

import (
	"encoding/json"
	"github.com/oracle/oci-go-sdk/common"
)

// NodeSourceOption The source option for the node.
type NodeSourceOption interface {

	// The user-friendly name of the entity corresponding to the OCID.
	GetSourceName() *string
}

type nodesourceoption struct {
	JsonData   []byte
	SourceName *string `mandatory:"false" json:"sourceName"`
	SourceType string  `json:"sourceType"`
}

// UnmarshalJSON unmarshals json
func (m *nodesourceoption) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalernodesourceoption nodesourceoption
	s := struct {
		Model Unmarshalernodesourceoption
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.SourceName = s.Model.SourceName
	m.SourceType = s.Model.SourceType

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *nodesourceoption) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.SourceType {
	case "IMAGE":
		mm := NodeSourceViaImageOption{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		return *m, nil
	}
}

//GetSourceName returns SourceName
func (m nodesourceoption) GetSourceName() *string {
	return m.SourceName
}

func (m nodesourceoption) String() string {
	return common.PointerString(m)
}
