package statsyncer

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/cluster-agent/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	syncInterval = 5 * time.Second
)

type StatSyncer struct {
	clusterName  string
	Clusters     v3.ClusterInterface
	ClusterNodes v3.MachineInterface
	Pods         corev1.PodInterface
	Nodes        corev1.NodeInterface
}

func Register(ctx context.Context, cluster *config.ClusterContext) {
	s := &StatSyncer{
		clusterName:  cluster.ClusterName,
		Clusters:     cluster.Management.Management.Clusters(""),
		ClusterNodes: cluster.Management.Management.Machines(""),
		Pods:         cluster.K8sClient.CoreV1().Pods(""),
		Nodes:        cluster.K8sClient.CoreV1().Nodes(),
	}

	go s.syncResources(ctx, syncInterval)
}

func (s *StatSyncer) syncResources(ctx context.Context, syncInterval time.Duration) {
	for range utils.TickerContext(ctx, syncInterval) {
		err := s.syncClusterNodeResources()
		logrus.Debugf("Syncing allocated resources for cluster [%s]", s.clusterName)
		if err != nil {
			logrus.Warn(err)
		}
	}
}

func (s *StatSyncer) syncClusterNodeResources() error {
	cluster, err := s.getCluster()
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Skip syncing node resources, cluster [%s] not found", s.clusterName)
			return nil
		}
		return err
	}
	if cluster == nil {
		logrus.Infof("Skip syncing node resources, cluster [%s] deleted", s.clusterName)
		return nil
	}
	if !utils.IsClusterProvisioned(cluster) {
		return fmt.Errorf("Skip syncing node resources - cluster [%s] not provisioned yet", s.clusterName)
	}
	nodes, err := s.Nodes.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("Skip syncing node resources - Error getting nodes %v", err)
	}
	cnodes, err := s.ClusterNodes.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("Skip syncing node resources - Error getting cluster nodes %v", err)
	}
	nodeNameToNode := s.getNodeMapping(nodes)

	return s.updateClusterNodeResources(cnodes, nodeNameToNode)
}

func (s *StatSyncer) updateClusterNodeResources(cnodes *v3.MachineList, nodeNameToNode map[string]*v1.Node) error {
	for _, cnode := range cnodes.Items {
		node := nodeNameToNode[cnode.Status.NodeName]
		if node == nil {
			logrus.Warnf("Skip adding cluster node resources [%s] Error getting Node %v", cnode.Name, cnode.Status.NodeName)
			continue
		}
		pods, err := s.getNonTerminatedPods(node.Name)
		if err != nil {
			logrus.Warnf("Skip adding cluster node resources [%s] Error getting Pods %v", node.Name, err)
			continue
		}
		var nodeData map[string]map[string]map[v1.ResourceName]resource.Quantity
		if pods != nil {
			//podName -> req/limit -> data
			nodeData = make(map[string]map[string]map[v1.ResourceName]resource.Quantity)
			for _, pod := range pods.Items {
				nodeData[pod.Name] = make(map[string]map[v1.ResourceName]resource.Quantity)
				requests, limits := s.getPodData(&pod)
				nodeData[pod.Name]["requests"] = requests
				nodeData[pod.Name]["limits"] = limits
			}
		}
		nodeRequests, nodeLimits := s.aggregate(nodeData)
		nodeRequests[v1.ResourcePods] = *resource.NewQuantity(int64(len(pods.Items)), resource.DecimalSI)
		if isClusterNodeChanged(&cnode, nodeRequests, nodeLimits) {
			err = s.updateClusterNode(&cnode, nodeRequests, nodeLimits)
			if err != nil {
				logrus.Warnf("Error updating cluster node resources [%s] %v", cnode.Name, err)
				continue
			}
		}
	}
	return nil
}

func isClusterNodeChanged(cnode *v3.Machine, requests map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) bool {
	return !isEqual(requests, cnode.Status.Requested) || !isEqual(limits, cnode.Status.Limits)
}

func (s *StatSyncer) updateClusterNode(cnode *v3.Machine, requests map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) error {
	if cnode.Status.Requested == nil {
		cnode.Status.Requested = v1.ResourceList{}
	}
	if cnode.Status.Limits == nil {
		cnode.Status.Limits = v1.ResourceList{}
	}

	for name, quantity := range requests {
		cnode.Status.Requested[name] = quantity
	}
	for name, quantity := range limits {
		cnode.Status.Limits[name] = quantity
	}

	_, err := s.ClusterNodes.Update(cnode)
	return err
}

func (s *StatSyncer) getNonTerminatedPods(nodeName string) (*v1.PodList, error) {
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + nodeName + ",status.phase!=Succeeded" + ",status.phase!=Failed")
	if err != nil {
		return nil, fmt.Errorf("Skip adding cluster node resources [%s] Error getting pods %v", nodeName, err)
	}
	nonTerminatedPodsList, err := s.Pods.List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
	if err != nil {
		if !errors.IsForbidden(err) {
			return nil, fmt.Errorf("Skip adding cluster node resources [%s] Access to pods forbidden %v", nodeName, err)
		}
	}
	return nonTerminatedPodsList, nil
}

func (s *StatSyncer) aggregate(data map[string]map[string]map[v1.ResourceName]resource.Quantity) (map[v1.ResourceName]resource.Quantity, map[v1.ResourceName]resource.Quantity) {
	requests, limits := map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	for _, podData := range data {
		podRequests, podLimits := podData["requests"], podData["limits"]
		addMap(podRequests, requests)
		addMap(podLimits, limits)
	}
	return requests, limits
}

func (s *StatSyncer) getCluster() (*v3.Cluster, error) {
	return s.Clusters.Get(s.clusterName, metav1.GetOptions{})
}

func (s *StatSyncer) getNodeMapping(nodes *v1.NodeList) map[string]*v1.Node {
	nodeNameToNode := make(map[string]*v1.Node)
	for _, node := range nodes.Items {
		nodeNameToNode[node.Name] = &node
	}
	return nodeNameToNode
}

func (s *StatSyncer) getPodData(pod *v1.Pod) (map[v1.ResourceName]resource.Quantity, map[v1.ResourceName]resource.Quantity) {
	requests, limits := map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
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

func isEqual(data1 map[v1.ResourceName]resource.Quantity, data2 map[v1.ResourceName]resource.Quantity) bool {
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

func addMap(data1 map[v1.ResourceName]resource.Quantity, data2 map[v1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		if value, ok := data2[name]; !ok {
			data2[name] = *quantity.Copy()
		} else {
			value.Add(quantity)
			data2[name] = value
		}
	}
}

func addMapForInit(data1 map[v1.ResourceName]resource.Quantity, data2 map[v1.ResourceName]resource.Quantity) {
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
