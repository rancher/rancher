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

// NodeSourceDetails The details of the node's source.
type NodeSourceDetails interface {
}

type nodesourcedetails struct {
	JsonData   []byte
	SourceType string `json:"sourceType"`
}

// UnmarshalJSON unmarshals json
func (m *nodesourcedetails) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalernodesourcedetails nodesourcedetails
	s := struct {
		Model Unmarshalernodesourcedetails
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.SourceType = s.Model.SourceType

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *nodesourcedetails) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.SourceType {
	case "IMAGE":
		mm := NodeSourceViaImageDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		return *m, nil
	}
}

func (m nodesourcedetails) String() string {
	return common.PointerString(m)
}
