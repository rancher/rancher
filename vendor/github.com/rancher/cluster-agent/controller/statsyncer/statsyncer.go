package statsyncer

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	syncInterval = 5 * time.Second
)

type PodsStatsSyncer struct {
	clusterName      string
	clusterNamespace string
	machines         v3.MachineInterface
}

type MachinesStatsSyncer struct {
	clusterName      string
	clusterNamespace string
	machines         v3.MachineInterface
	pods             v1.PodInterface
}

func Register(cluster *config.ClusterContext) {
	sc := &PodsStatsSyncer{
		clusterName:      cluster.ClusterName,
		clusterNamespace: cluster.ClusterName,
		machines:         cluster.Management.Management.Machines(cluster.ClusterName),
	}

	sy := &MachinesStatsSyncer{
		clusterName:      cluster.ClusterName,
		clusterNamespace: cluster.ClusterName,
		machines:         cluster.Management.Management.Machines(cluster.ClusterName),
		pods:             cluster.Core.Pods(""),
	}

	cluster.Core.Pods("").AddLifecycle("podstats-syncer", sc)
	cluster.Management.Management.Machines(cluster.ClusterName).Controller().AddHandler("machinestats-syncer", sy.sync)
}

func (s *PodsStatsSyncer) Create(pod *corev1.Pod) (*corev1.Pod, error) {
	s.updateMachine(pod)
	return nil, nil
}

func (s *PodsStatsSyncer) Updated(pod *corev1.Pod) (*corev1.Pod, error) {
	// update shouldn't trigger anything, as nodeName is set in the spec
	return nil, nil
}

func (s *PodsStatsSyncer) Remove(pod *corev1.Pod) (*corev1.Pod, error) {
	s.updateMachine(pod)
	return nil, nil
}

func (s *PodsStatsSyncer) updateMachine(pod *corev1.Pod) {
	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		return
	}

	machine, err := s.getMachine(nodeName)
	if err != nil || machine == nil {
		logrus.Warnf("Failed to get machine for pod [%s] on node [%s]: %v", pod.Name, nodeName, err)
		return
	}

	s.machines.Controller().Enqueue(machine.Namespace, machine.Name)
}

func (s *MachinesStatsSyncer) sync(key string, machine *v3.Machine) error {
	if machine == nil || machine.DeletionTimestamp != nil {
		return nil
	}

	nodeName := machine.Status.NodeName
	if nodeName == "" {
		if machine.Status.NodeConfig != nil {
			nodeName = machine.Status.NodeConfig.HostnameOverride
		}
	}

	if nodeName == "" {
		logrus.Warnf("Failed to get nodeName from machine [%s]", machine.Name)
		// can't really do anything in this case
		return nil
	}
	return s.updateMachineResources(machine, nodeName)
}

func (s *MachinesStatsSyncer) updateMachineResources(machine *v3.Machine, nodeName string) error {
	pods, err := s.getNonTerminatedPods(nodeName)
	if err != nil {
		return err
	}
	var nodeData map[string]map[string]map[corev1.ResourceName]resource.Quantity
	if pods != nil {
		//podName -> req/limit -> data
		nodeData = make(map[string]map[string]map[corev1.ResourceName]resource.Quantity)
		for _, pod := range pods {
			nodeData[pod.Name] = make(map[string]map[corev1.ResourceName]resource.Quantity)
			requests, limits := s.getPodData(&pod)
			nodeData[pod.Name]["requests"] = requests
			nodeData[pod.Name]["limits"] = limits
		}
	}
	nodeRequests, nodeLimits := s.aggregate(nodeData)
	nodeRequests[corev1.ResourcePods] = *resource.NewQuantity(int64(len(pods)), resource.DecimalSI)
	if machineChanged(machine, nodeRequests, nodeLimits) {
		toUpdate := machine.DeepCopy()
		err = s.updateClusterNode(toUpdate, nodeRequests, nodeLimits)
		if err != nil {
			return err
		}
	}
	return nil
}

func machineChanged(cnode *v3.Machine, requests map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity) bool {
	return !isEqual(requests, cnode.Status.Requested) || !isEqual(limits, cnode.Status.Limits)
}

func (s *MachinesStatsSyncer) updateClusterNode(cnode *v3.Machine, requests map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity) error {
	if cnode.Status.Requested == nil {
		cnode.Status.Requested = corev1.ResourceList{}
	}
	if cnode.Status.Limits == nil {
		cnode.Status.Limits = corev1.ResourceList{}
	}

	for name, quantity := range requests {
		cnode.Status.Requested[name] = quantity
	}
	for name, quantity := range limits {
		cnode.Status.Limits[name] = quantity
	}

	_, err := s.machines.Update(cnode)
	return err
}

func (s *MachinesStatsSyncer) getNonTerminatedPods(nodeName string) ([]corev1.Pod, error) {
	//TODO switch pods to list from cache once cache implements field selector
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + nodeName + ",status.phase!=Succeeded" + ",status.phase!=Failed")
	if err != nil {
		return nil, fmt.Errorf("Skip adding cluster node resources [%s] Error getting pods %v", nodeName, err)
	}
	var toReturn []corev1.Pod
	nonTerminatedPods, err := s.pods.List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch the pods")
	}

	for _, pod := range nonTerminatedPods.Items {
		if pod.DeletionTimestamp == nil {
			toReturn = append(toReturn, pod)
		}
	}
	return toReturn, nil
}

func (s *MachinesStatsSyncer) aggregate(data map[string]map[string]map[corev1.ResourceName]resource.Quantity) (map[corev1.ResourceName]resource.Quantity, map[corev1.ResourceName]resource.Quantity) {
	requests, limits := map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, podData := range data {
		podRequests, podLimits := podData["requests"], podData["limits"]
		addMap(podRequests, requests)
		addMap(podLimits, limits)
	}
	return requests, limits
}

func (s *MachinesStatsSyncer) getNodeMapping(nodes []*corev1.Node) map[string]*corev1.Node {
	nodeNameToNode := make(map[string]*corev1.Node)
	for _, node := range nodes {
		nodeNameToNode[node.Name] = node
	}
	return nodeNameToNode
}

func (s *MachinesStatsSyncer) getPodData(pod *corev1.Pod) (map[corev1.ResourceName]resource.Quantity, map[corev1.ResourceName]resource.Quantity) {
	requests, limits := map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, container := range pod.Spec.Containers {
		addMap(container.Resources.Requests, requests)
		addMap(container.Resources.Limits, limits)
	}

	for _, container := range pod.Spec.InitContainers {
		addMapForInit(container.Resources.Requests, requests)
		addMapForInit(container.Resources.Limits, limits)
	}
	return requests, limits
}

func isEqual(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) bool {
	if data1 == nil && data2 == nil {
		return true
	}
	if data1 == nil || data2 == nil {
		return false
	}
	for key, value := range data1 {
		if _, exists := data2[key]; !exists {
			return false
		}
		value2 := data2[key]
		if value.Value() != value2.Value() {
			return false
		}
	}
	return true
}

func addMap(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		if value, ok := data2[name]; !ok {
			data2[name] = *quantity.Copy()
		} else {
			value.Add(quantity)
			data2[name] = value
		}
	}
}

func addMapForInit(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		value, ok := data2[name]
		if !ok {
			data2[name] = *quantity.Copy()
			continue
		}
		if quantity.Cmp(value) > 0 {
			data2[name] = *quantity.Copy()
		}
	}
}

func (s *PodsStatsSyncer) getMachine(nodeName string) (*v3.Machine, error) {
	machines, err := s.machines.Controller().Lister().List(s.clusterNamespace, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if machine.Status.NodeName == nodeName {
			return machine, nil
		}
		// to handle the case when machine was provisioned first
		if machine.Status.NodeConfig != nil {
			if machine.Status.NodeConfig.HostnameOverride == nodeName {
				return machine, nil
			}
		}
	}

	return nil, nil
}
