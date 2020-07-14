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

// NodeSourceViaImageOption An image can be specified as the source of nodes when launching a node pool using the `nodeSourceDetails` object.
type NodeSourceViaImageOption struct {

	// The user-friendly name of the entity corresponding to the OCID.
	SourceName *string `mandatory:"false" json:"sourceName"`

	// The OCID of the image.
	ImageId *string `mandatory:"false" json:"imageId"`
}

//GetSourceName returns SourceName
func (m NodeSourceViaImageOption) GetSourceName() *string {
	return m.SourceName
}

func (m NodeSourceViaImageOption) String() string {
	return common.PointerString(m)
}

// MarshalJSON marshals to json representation
func (m NodeSourceViaImageOption) MarshalJSON() (buff []byte, e error) {
	type MarshalTypeNodeSourceViaImageOption NodeSourceViaImageOption
	s := struct {
		DiscriminatorParam string `json:"sourceType"`
		MarshalTypeNodeSourceViaImageOption
	}{
		"IMAGE",
		(MarshalTypeNodeSourceViaImageOption)(m),
	}

	return json.Marshal(&s)
}
