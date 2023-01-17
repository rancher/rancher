package rke1

import (
	"strconv"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"k8s.io/apimachinery/pkg/util/wait"
)

// IsNodeReady is a helper method that will loop and check if the node is ready in the RKE1 cluster.
// It will return an error if the node is not ready after set amount of time.
func IsNodeReady(client *rancher.Client, nodePool *management.NodePool, ClusterID string) error {
	err := wait.Poll(500*time.Millisecond, 30*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId":  ClusterID,
				"nodePoolId": nodePool.ID,
			},
		})
		if err != nil {
			return false, err
		}

		const active = "active"

		for _, node := range nodes.Data {
			node, err := client.Management.Node.ByID(node.ID)
			if err != nil {
				return false, err
			}

			if node.State == active {
				return true, nil
			}

			return false, nil
		}
		if err != nil {
			return false, err
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

type NodeRoles struct {
	ControlPlane bool  `json:"controlplane,omitempty" yaml:"controlplane,omitempty"`
	Etcd         bool  `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Worker       bool  `json:"worker,omitempty" yaml:"worker,omitempty"`
	Quantity     int64 `json:"quantity" yaml:"quantity"`
}

// NodePoolSetup is a helper method that will loop and setup muliple node pools with the defined node roles from the `nodeRoles` parameter
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
func NodePoolSetup(client *rancher.Client, nodeRoles []NodeRoles, ClusterID, NodeTemplateID string) (*management.NodePool, error) {
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
		nodePoolConfig.HostnamePrefix = "auto-rke1-" + strconv.Itoa(index) + ClusterID

		_, err := client.Management.NodePool.Create(&nodePoolConfig)

		if err != nil {
			return nil, err
		}
	}

	return &nodePoolConfig, nil
}

// ScaleWorkerNodePool is a helper method that will add a worker node pool to the existing RKE1 cluster. Once done, it will scale
// the worker node pool to add a worker node, scale it back down to remove the worker node, and then delete the worker node pool.
func ScaleWorkerNodePool(client *rancher.Client, nodeRoles []NodeRoles, ClusterID, NodeTemplateID string) error {
	nodePoolConfig := management.NodePool{
		ClusterID:               ClusterID,
		ControlPlane:            false,
		DeleteNotReadyAfterSecs: 0,
		Etcd:                    false,
		HostnamePrefix:          "auto-rke1-scale-test",
		NodeTemplateID:          NodeTemplateID,
		Quantity:                1,
		Worker:                  true,
	}

	nodePool, err := client.Management.NodePool.Create(&nodePoolConfig)
	if err != nil {
		return err
	}

	IsNodeReady(client, nodePool, ClusterID)

	nodePoolConfig.Quantity = 2

	updatedPool, err := client.Management.NodePool.Update(nodePool, &nodePoolConfig)
	if err != nil {
		return err
	}

	IsNodeReady(client, updatedPool, ClusterID)

	nodePoolConfig.Quantity = 1

	_, err = client.Management.NodePool.Update(updatedPool, &nodePoolConfig)
	if err != nil {
		return err
	}

	err = client.Management.NodePool.Delete(nodePool)
	if err != nil {
		return err
	}

	return nil
}
