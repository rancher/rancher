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

// NodePool A pool of compute nodes attached to a cluster. Avoid entering confidential information.
type NodePool struct {

	// The OCID of the node pool.
	Id *string `mandatory:"false" json:"id"`

	// The OCID of the compartment in which the node pool exists.
	CompartmentId *string `mandatory:"false" json:"compartmentId"`

	// The OCID of the cluster to which this node pool is attached.
	ClusterId *string `mandatory:"false" json:"clusterId"`

	// The name of the node pool.
	Name *string `mandatory:"false" json:"name"`

	// The version of Kubernetes running on the nodes in the node pool.
	KubernetesVersion *string `mandatory:"false" json:"kubernetesVersion"`

	// A list of key/value pairs to add to each underlying OCI instance in the node pool.
	NodeMetadata map[string]string `mandatory:"false" json:"nodeMetadata"`

	// Deprecated. see `nodeSource`. The OCID of the image running on the nodes in the node pool.
	NodeImageId *string `mandatory:"false" json:"nodeImageId"`

	// Deprecated. see `nodeSource`. The name of the image running on the nodes in the node pool.
	NodeImageName *string `mandatory:"false" json:"nodeImageName"`

	// Source running on the nodes in the node pool.
	NodeSource NodeSourceOption `mandatory:"false" json:"nodeSource"`

	// The name of the node shape of the nodes in the node pool.
	NodeShape *string `mandatory:"false" json:"nodeShape"`

	// A list of key/value pairs to add to nodes after they join the Kubernetes cluster.
	InitialNodeLabels []KeyValue `mandatory:"false" json:"initialNodeLabels"`

	// The SSH public key on each node in the node pool.
	SshPublicKey *string `mandatory:"false" json:"sshPublicKey"`

	// The number of nodes in each subnet.
	QuantityPerSubnet *int `mandatory:"false" json:"quantityPerSubnet"`

	// The OCIDs of the subnets in which to place nodes for this node pool.
	SubnetIds []string `mandatory:"false" json:"subnetIds"`

	// The nodes in the node pool.
	Nodes []Node `mandatory:"false" json:"nodes"`

	// The configuration of nodes in the node pool.
	NodeConfigDetails *NodePoolNodeConfigDetails `mandatory:"false" json:"nodeConfigDetails"`
}

func (m NodePool) String() string {
	return common.PointerString(m)
}

// UnmarshalJSON unmarshals from json
func (m *NodePool) UnmarshalJSON(data []byte) (e error) {
	model := struct {
		Id                *string                    `json:"id"`
		CompartmentId     *string                    `json:"compartmentId"`
		ClusterId         *string                    `json:"clusterId"`
		Name              *string                    `json:"name"`
		KubernetesVersion *string                    `json:"kubernetesVersion"`
		NodeMetadata      map[string]string          `json:"nodeMetadata"`
		NodeImageId       *string                    `json:"nodeImageId"`
		NodeImageName     *string                    `json:"nodeImageName"`
		NodeSource        nodesourceoption           `json:"nodeSource"`
		NodeShape         *string                    `json:"nodeShape"`
		InitialNodeLabels []KeyValue                 `json:"initialNodeLabels"`
		SshPublicKey      *string                    `json:"sshPublicKey"`
		QuantityPerSubnet *int                       `json:"quantityPerSubnet"`
		SubnetIds         []string                   `json:"subnetIds"`
		Nodes             []Node                     `json:"nodes"`
		NodeConfigDetails *NodePoolNodeConfigDetails `json:"nodeConfigDetails"`
	}{}

	e = json.Unmarshal(data, &model)
	if e != nil {
		return
	}
	var nn interface{}
	m.Id = model.Id

	m.CompartmentId = model.CompartmentId

	m.ClusterId = model.ClusterId

	m.Name = model.Name

	m.KubernetesVersion = model.KubernetesVersion

	m.NodeMetadata = model.NodeMetadata

	m.NodeImageId = model.NodeImageId

	m.NodeImageName = model.NodeImageName

	nn, e = model.NodeSource.UnmarshalPolymorphicJSON(model.NodeSource.JsonData)
	if e != nil {
		return
	}
	if nn != nil {
		m.NodeSource = nn.(NodeSourceOption)
	} else {
		m.NodeSource = nil
	}

	m.NodeShape = model.NodeShape

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

	m.Nodes = make([]Node, len(model.Nodes))
	for i, n := range model.Nodes {
		m.Nodes[i] = n
	}

	m.NodeConfigDetails = model.NodeConfigDetails
	return
}
