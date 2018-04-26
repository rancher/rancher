package node

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
)

const (
	externalAddressAnnotation = "rke.cattle.io/external-ip"
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

func IsNodeForNode(node *corev1.Node, machine *v3.Node) bool {
	nodeName := GetNodeName(machine)
	if nodeName == node.Name {
		return true
	}

	machineAddress := ""
	if machine.Status.NodeConfig != nil {
		if machine.Status.NodeConfig.InternalAddress == "" {
			// rke defaults internal address to address
			machineAddress = machine.Status.NodeConfig.Address
		} else {
			machineAddress = machine.Status.NodeConfig.InternalAddress
		}
	}

	if machineAddress == "" {
		return false
	}

	if machineAddress == getNodeInternalAddress(node) {
		return true
	}

	return false
}

func getNodeInternalAddress(node *corev1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address
		}
	}
	return ""
}

func GetEndpointNodeIP(node *v3.Node) string {
	externalIP := ""
	internalIP := ""
	for _, ip := range node.Status.InternalNodeStatus.Addresses {
		if ip.Type == "ExternalIP" && ip.Address != "" {
			externalIP = ip.Address
			break
		} else if ip.Type == "InternalIP" && ip.Address != "" {
			internalIP = ip.Address
		}
	}
	if externalIP != "" {
		return externalIP
	}
	if node.Annotations != nil {
		externalIP = node.Status.NodeAnnotations[externalAddressAnnotation]
		if externalIP != "" {
			return externalIP
		}
	}
	return internalIP
}

func GetNodeByNodeName(nodes []*v3.Node, nodeName string) *v3.Node {
	for _, m := range nodes {
		if GetNodeName(m) == nodeName {
			return m
		}
	}
	return nil
}
