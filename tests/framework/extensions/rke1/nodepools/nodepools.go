package rke1

import (
	"strconv"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/sirupsen/logrus"
)

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
		HostnamePrefix:          "auto-rke1-scale-" + ClusterID,
		NodeTemplateID:          NodeTemplateID,
		Quantity:                1,
		Worker:                  true,
	}

	logrus.Infof("Creating new worker node pool...")
	nodePool, err := client.Management.NodePool.Create(&nodePoolConfig)
	if err != nil {
		return err
	}

	if nodestat.AllManagementNodeReady(client, ClusterID) != nil {
		return err
	}

	logrus.Infof("New node pool is ready!")
	nodePoolConfig.Quantity = 2

	logrus.Infof("Scaling node pool to 2 worker nodes...")
	updatedPool, err := client.Management.NodePool.Update(nodePool, &nodePoolConfig)
	if err != nil {
		return err
	}

	if nodestat.AllManagementNodeReady(client, ClusterID) != nil {
		return err
	}

	logrus.Infof("Node pool is scaled to 2 worker nodes!")
	nodePoolConfig.Quantity = 1

	logrus.Infof("Scaling node pool back to 1 worker node...")
	_, err = client.Management.NodePool.Update(updatedPool, &nodePoolConfig)
	if err != nil {
		return err
	}

	logrus.Infof("Node pool is scaled back to 1 worker node!")

	logrus.Infof("Deleting node pool...")
	err = client.Management.NodePool.Delete(nodePool)
	if err != nil {
		return err
	}

	logrus.Infof("Node pool deleted!")

	return nil
}
