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
	"github.com/oracle/oci-go-sdk/common"
)

// CreateNodePoolNodeConfigDetails The size and placement configuration of nodes in the node pool.
type CreateNodePoolNodeConfigDetails struct {

	// The number of nodes that should be in the node pool.
	Size *int `mandatory:"true" json:"size"`

	// The placement configurations for the node pool. Provide one placement
	// configuration for each availability domain in which you intend to launch a node.
	// To use the node pool with a regional subnet, provide a placement configuration for
	// each availability domain, and include the regional subnet in each placement
	// configuration.
	PlacementConfigs []NodePoolPlacementConfigDetails `mandatory:"true" json:"placementConfigs"`
}

func (m CreateNodePoolNodeConfigDetails) String() string {
	return common.PointerString(m)
}
