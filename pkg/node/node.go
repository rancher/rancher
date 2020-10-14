package node

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/settings"
	rkeservices "github.com/rancher/rke/services"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	externalAddressAnnotation = "rke.cattle.io/external-ip"
	LabelNodeName             = "management.cattle.io/nodename"
	nodeStatusLabel           = "cattle.rancher.io/node-status"
)

func GetNodeName(machine *v3.Node) string {
	if machine.Status.NodeName != "" {
		return machine.Status.NodeName
	}
	// to handle the case when machine was provisioned first
	if machine.Status.NodeConfig != nil {
		if machine.Status.NodeConfig.HostnameOverride != "" {
			return machine.Status.NodeConfig.HostnameOverride
		}
	}
	return ""
}

func IgnoreNode(name string, labels map[string]string) bool {
	ignoreName := settings.IgnoreNodeName.Get()
	if name == ignoreName {
		return true
	}

	if labels == nil {
		return false
	}
	value, ok := labels[nodeStatusLabel]
	return ok && value == "ignore"
}

// IsNodeForNode returns true if node names or addresses are equal
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

	if machineAddress == GetNodeInternalAddress(node) {
		return true
	}

	return false
}

func GetNodeInternalAddress(node *corev1.Node) string {
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

func GetNodeForMachine(machine *v3.Node, nodeLister v1.NodeLister) (*corev1.Node, error) {
	nodeName := ""
	if machine.Labels != nil {
		nodeName = machine.Labels[LabelNodeName]
	}
	var nodes []*corev1.Node
	var err error
	if nodeName != "" {
		var node *corev1.Node
		node, err = nodeLister.Get("", nodeName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err

		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) == 0 {
		nodes, err = nodeLister.List("", labels.NewSelector())
		if err != nil {
			return nil, err
		}
	}

	for _, n := range nodes {
		if IsNodeForNode(n, machine) {
			return n, nil
		}
	}

	return nil, nil
}

func GetMachineForNode(node *corev1.Node, clusterNamespace string, machineLister v3.NodeLister) (*v3.Node, error) {
	labelsSearchSet := labels.Set{LabelNodeName: node.Name}
	machines, err := machineLister.List(clusterNamespace, labels.SelectorFromSet(labelsSearchSet))
	if err != nil {
		return nil, err
	}
	if len(machines) == 0 {
		machines, err = machineLister.List(clusterNamespace, labels.NewSelector())
		if err != nil {
			return nil, err
		}
	}
	for _, machine := range machines {
		if IsNodeForNode(node, machine) {
			return machine, nil
		}
	}
	return nil, nil
}

func IsMachineReady(machine *v3.Node) bool {
	for _, cond := range machine.Status.InternalNodeStatus.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func DrainBeforeDelete(node *v3.Node, cluster *v3.Cluster) bool {
	// never drain an etcd node, you will regret it
	if IsEtcd(node.Status.NodeConfig) {
		return false
	}

	// vsphere nodes with cloud providers need to be drained to unmount vmdk files as vsphere deletes them automatically
	if nil != cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider {
		return true
	}

	return false
}

func IsEtcd(node *v3.RKEConfigNode) bool {
	roles := node.Role
	for _, role := range roles {
		if role == rkeservices.ETCDRole {
			return true
		}
	}
	return false
}

func IsControlPlane(node *v3.RKEConfigNode) bool {
	for _, role := range node.Role {
		if role == rkeservices.ControlRole {
			return true
		}
	}
	return false
}

func IsWorker(node *v3.RKEConfigNode) bool {
	roles := node.Role
	for _, role := range roles {
		if role == rkeservices.WorkerRole {
			return true
		}
	}
	return false
}

func IsNonWorker(node *v3.RKEConfigNode) bool {
	return IsControlPlane(node) || IsEtcd(node)
}

func GetDrainFlags(node *v3.Node) []string {
	input := node.Spec.NodeDrainInput
	if input == nil {
		return []string{}
	}

	var ignoreDaemonSets bool
	if input.IgnoreDaemonSets == nil {
		ignoreDaemonSets = true
	} else {
		ignoreDaemonSets = *input.IgnoreDaemonSets
	}
	return []string{
		fmt.Sprintf("--delete-local-data=%v", input.DeleteLocalData),
		fmt.Sprintf("--force=%v", input.Force),
		fmt.Sprintf("--grace-period=%v", input.GracePeriod),
		fmt.Sprintf("--ignore-daemonsets=%v", ignoreDaemonSets),
		fmt.Sprintf("--timeout=%s", convert.ToString(input.Timeout)+"s")}
}

func HasFinalizerWithPrefix(node *v3.Node, prefix string) bool {
	for _, finalizer := range node.Finalizers {
		if strings.HasPrefix(finalizer, prefix) {
			return true
		}
	}
	return false
}

func RemoveFinalizerWithPrefix(node *v3.Node, prefix string) *v3.Node {
	var newFinalizers []string
	for _, finalizer := range node.Finalizers {
		if strings.HasPrefix(finalizer, prefix) {
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}
	node.SetFinalizers(newFinalizers)
	return node
}
