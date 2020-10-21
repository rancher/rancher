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

// CreateNodePoolDetails The properties that define a request to create a node pool.
type CreateNodePoolDetails struct {

	// The OCID of the compartment in which the node pool exists.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The OCID of the cluster to which this node pool is attached.
	ClusterId *string `mandatory:"true" json:"clusterId"`

	// The name of the node pool. Avoid entering confidential information.
	Name *string `mandatory:"true" json:"name"`

	// The version of Kubernetes to install on the nodes in the node pool.
	KubernetesVersion *string `mandatory:"true" json:"kubernetesVersion"`

	// The name of the node shape of the nodes in the node pool.
	NodeShape *string `mandatory:"true" json:"nodeShape"`

	// A list of key/value pairs to add to each underlying OCI instance in the node pool.
	NodeMetadata map[string]string `mandatory:"false" json:"nodeMetadata"`

	// Deprecated. Use `nodeSourceDetails` instead.
	// If you specify values for both, this value is ignored.
	// The name of the image running on the nodes in the node pool.
	NodeImageName *string `mandatory:"false" json:"nodeImageName"`

	// Specify the source to use to launch nodes in the node pool. Currently, image is the only supported source.
	NodeSourceDetails NodeSourceDetails `mandatory:"false" json:"nodeSourceDetails"`

	// A list of key/value pairs to add to nodes after they join the Kubernetes cluster.
	InitialNodeLabels []KeyValue `mandatory:"false" json:"initialNodeLabels"`

	// The SSH public key to add to each node in the node pool.
	SshPublicKey *string `mandatory:"false" json:"sshPublicKey"`

	// Optional, default to 1. The number of nodes to create in each subnet specified in subnetIds property.
	// When used, subnetIds is required. This property is deprecated, use nodeConfigDetails instead.
	QuantityPerSubnet *int `mandatory:"false" json:"quantityPerSubnet"`

	// The OCIDs of the subnets in which to place nodes for this node pool. When used, quantityPerSubnet
	// can be provided. This property is deprecated, use nodeConfigDetails. Exactly one of the
	// subnetIds or nodeConfigDetails properties must be specified.
	SubnetIds []string `mandatory:"false" json:"subnetIds"`

	// The configuration of nodes in the node pool. Exactly one of the
	// subnetIds or nodeConfigDetails properties must be specified.
	NodeConfigDetails *CreateNodePoolNodeConfigDetails `mandatory:"false" json:"nodeConfigDetails"`
}

func (m CreateNodePoolDetails) String() string {
	return common.PointerString(m)
}

// UnmarshalJSON unmarshals from json
func (m *CreateNodePoolDetails) UnmarshalJSON(data []byte) (e error) {
	model := struct {
		NodeMetadata      map[string]string                `json:"nodeMetadata"`
		NodeImageName     *string                          `json:"nodeImageName"`
		NodeSourceDetails nodesourcedetails                `json:"nodeSourceDetails"`
		InitialNodeLabels []KeyValue                       `json:"initialNodeLabels"`
		SshPublicKey      *string                          `json:"sshPublicKey"`
		QuantityPerSubnet *int                             `json:"quantityPerSubnet"`
		SubnetIds         []string                         `json:"subnetIds"`
		NodeConfigDetails *CreateNodePoolNodeConfigDetails `json:"nodeConfigDetails"`
		CompartmentId     *string                          `json:"compartmentId"`
		ClusterId         *string                          `json:"clusterId"`
		Name              *string                          `json:"name"`
		KubernetesVersion *string                          `json:"kubernetesVersion"`
		NodeShape         *string                          `json:"nodeShape"`
	}{}

	e = json.Unmarshal(data, &model)
	if e != nil {
		return
	}
	var nn interface{}
	m.NodeMetadata = model.NodeMetadata

	m.NodeImageName = model.NodeImageName

	nn, e = model.NodeSourceDetails.UnmarshalPolymorphicJSON(model.NodeSourceDetails.JsonData)
	if e != nil {
		return
	}
	if nn != nil {
		m.NodeSourceDetails = nn.(NodeSourceDetails)
	} else {
		m.NodeSourceDetails = nil
	}

	m.InitialNodeLabels = make([]KeyValue, len(model.InitialNodeLabels))
	for i, n := range model.InitialNodeLabels {
		m.InitialNodeLabels[i] = n
	}

	m.SshPublicKey = model.SshPublicKey

	m.QuantityPerSubnet = model.QuantityPerSubnet

	m.SubnetIds = make([]string, len(model.SubnetIds))
	for i, n := range model.SubnetIds {
		m.SubnetIds[i] = n
	}

	m.NodeConfigDetails = model.NodeConfigDetails

	m.CompartmentId = model.CompartmentId

	m.ClusterId = model.ClusterId

	m.Name = model.Name

	m.KubernetesVersion = model.KubernetesVersion

	m.NodeShape = model.NodeShape
	return
}
