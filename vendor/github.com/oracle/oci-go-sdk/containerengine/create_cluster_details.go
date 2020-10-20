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

// CreateClusterDetails The properties that define a request to create a cluster.
type CreateClusterDetails struct {

	// The name of the cluster. Avoid entering confidential information.
	Name *string `mandatory:"true" json:"name"`

	// The OCID of the compartment in which to create the cluster.
	CompartmentId *string `mandatory:"true" json:"compartmentId"`

	// The OCID of the virtual cloud network (VCN) in which to create the cluster.
	VcnId *string `mandatory:"true" json:"vcnId"`

	// The version of Kubernetes to install into the cluster masters.
	KubernetesVersion *string `mandatory:"true" json:"kubernetesVersion"`

	// The OCID of the KMS key to be used as the master encryption key for Kubernetes secret encryption.
	// When used, `kubernetesVersion` must be at least `v1.13.0`.
	KmsKeyId *string `mandatory:"false" json:"kmsKeyId"`

	// Optional attributes for the cluster.
	Options *ClusterCreateOptions `mandatory:"false" json:"options"`
}

func (m CreateClusterDetails) String() string {
	return common.PointerString(m)
}
