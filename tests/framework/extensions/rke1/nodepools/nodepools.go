package rke1

import (
	"strconv"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

type NodeRoles struct {
	ControlPlane bool  `json:"controlplane,omitempty" yaml:"controlplane,omitempty"`
	Etcd         bool  `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Worker       bool  `json:"worker,omitempty" yaml:"worker,omitempty"`
	Quantity     int64 `json:"quantity" yaml:"quantity"`
}

// RKE1NodePoolSetup is a helper method that will loop and setup muliple node pools with the defined node roles from the `nodeRoles` parameter
// `nodeRoles` would be in this format
//
//	  []map[string]bool{
//	  {
//		   ControlPlane: true,
//		   Etcd:         false,
//		   Worker:       false,
//		   Quantity:     1,
//	  },
//	  {
//		   ControlPlane: false,
//		   Etcd:         true,
//		   Worker:       false,
//		   Quantity:     1,
//	  },
//	 }
func RKE1NodePoolSetup(client *rancher.Client, nodeRoles []NodeRoles, ClusterID, NodeTemplateID string) (*management.NodePool, error) {
	nodePoolConfig := management.NodePool{
		ClusterID:               ClusterID,
		DeleteNotReadyAfterSecs: 0,
		NodeTemplateID:          NodeTemplateID,
	}

	for index, roles := range nodeRoles {
		nodePoolConfig.ControlPlane = roles.ControlPlane
		nodePoolConfig.Etcd = roles.Etcd
		nodePoolConfig.Worker = roles.Worker
		nodePoolConfig.Quantity = roles.Quantity
		nodePoolConfig.HostnamePrefix = "auto-rke1-" + strconv.Itoa(index)

		_, err := client.Management.NodePool.Create(&nodePoolConfig)

		if err != nil {
			return nil, err
		}
	}

	return &nodePoolConfig, nil
}
