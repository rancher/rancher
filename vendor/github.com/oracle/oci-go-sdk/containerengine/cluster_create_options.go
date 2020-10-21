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

// ClusterCreateOptions The properties that define extra options for a cluster.
type ClusterCreateOptions struct {

	// The OCIDs of the subnets used for Kubernetes services load balancers.
	ServiceLbSubnetIds []string `mandatory:"false" json:"serviceLbSubnetIds"`

	// Network configuration for Kubernetes.
	KubernetesNetworkConfig *KubernetesNetworkConfig `mandatory:"false" json:"kubernetesNetworkConfig"`

	// Configurable cluster add-ons
	AddOns *AddOnOptions `mandatory:"false" json:"addOns"`

	// Configurable cluster admission controllers
	AdmissionControllerOptions *AdmissionControllerOptions `mandatory:"false" json:"admissionControllerOptions"`
}

func (m ClusterCreateOptions) String() string {
	return common.PointerString(m)
}
