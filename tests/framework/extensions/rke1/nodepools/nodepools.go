package rke1

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	rke1 "github.com/rancher/rancher/tests/framework/extensions/clusters"
)

func NewRKE1NodePoolConfig(ClusterID string, NodeTemplateID string) *management.NodePool {
	nodePoolConfig := &management.NodePool{
		ClusterID:               ClusterID,
		ControlPlane:            true,
		DeleteNotReadyAfterSecs: 0,
		Etcd:                    true,
		HostnamePrefix:          rke1.RKE1AppendRandomString("rke1"),
		NodeTemplateID:          NodeTemplateID,
		Quantity:                1,
		Worker:                  true,
	}

	return nodePoolConfig
}
