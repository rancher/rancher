package node

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func GetNodeName(node *v3.Node) string {
	if node.Status.NodeName != "" {
		return node.Status.NodeName
	}
	// to handle the case when node was provisioned first
	if node.Status.NodeConfig != nil {
		if node.Status.NodeConfig.HostnameOverride != "" {
			return node.Status.NodeConfig.HostnameOverride
		}
	}
	return ""
}
