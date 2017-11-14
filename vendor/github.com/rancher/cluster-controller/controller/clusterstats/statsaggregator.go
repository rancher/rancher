package clusterstats

import (
	"encoding/json"
	"time"

	clusterv1 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatsAggregator struct {
	Clusters clusterv1.ClusterInterface
}

type ClusterNodeData struct {
	Capacity                        v1.ResourceList
	Allocatable                     v1.ResourceList
	Requested                       v1.ResourceList
	Limits                          v1.ResourceList
	ConditionNoDiskPressureStatus   v1.ConditionStatus
	ConditionNoMemoryPressureStatus v1.ConditionStatus
}

var stats map[string]map[string]*ClusterNodeData
var nodeNameToClusterName map[string]string

func Register(cluster *config.ManagementContext) {
	stats = make(map[string]map[string]*ClusterNodeData)
	nodeNameToClusterName = make(map[string]string)
	s := &StatsAggregator{
		Clusters: cluster.Management.Clusters(""),
	}
	cluster.Management.Machines("").Controller().AddHandler(s.sync)
}

func (s *StatsAggregator) sync(key string, clusterNode *clusterv1.Machine) error {
	logrus.Infof("Syncing clusternode [%s]", key)
	if clusterNode == nil {
		return s.deleteStats(key)
	}
	return s.addOrUpdateStats(clusterNode)
}

func (s *StatsAggregator) deleteStats(key string) error {
	if _, exists := nodeNameToClusterName[key]; !exists {
		logrus.Infof("ClusterNode [%s] already deleted from stats", key)
		return nil
	}
	clusterName, clusterNodeName := nodeNameToClusterName[key], key
	cluster, err := s.getCluster(clusterName)
	if err != nil {
		return err
	}
	oldData := stats[clusterName][clusterNodeName]
	if _, exists := stats[clusterName][clusterNodeName]; exists {
		delete(stats[clusterName], clusterNodeName)
		delete(nodeNameToClusterName, clusterNodeName)
		logrus.Infof("ClusterNode [%s] stats deleted", key)
	}
	s.aggregate(cluster, clusterName)
	err = s.update(cluster)
	if err != nil {
		stats[clusterName][clusterNodeName] = oldData
		return err
	}
	logrus.Infof("Successfully updated cluster [%s] stats", clusterName)
	return nil
}

func (s *StatsAggregator) addOrUpdateStats(clusterNode *clusterv1.Machine) error {
	clusterName, clusterNodeName := clusterNode.Spec.ClusterName, clusterNode.Status.NodeName
	cluster, err := s.getCluster(clusterName)
	if err != nil {
		return err
	}
	if _, exists := stats[clusterName]; !exists {
		stats[clusterName] = make(map[string]*ClusterNodeData)
	}

	oldData := stats[clusterName][clusterNodeName]
	newData := &ClusterNodeData{
		Capacity:    clusterNode.Status.NodeStatus.Capacity,
		Allocatable: clusterNode.Status.NodeStatus.Allocatable,
		Requested:   clusterNode.Status.Requested,
		Limits:      clusterNode.Status.Limits,
		ConditionNoDiskPressureStatus:   getNodeConditionByType(clusterNode.Status.NodeStatus.Conditions, v1.NodeDiskPressure).Status,
		ConditionNoMemoryPressureStatus: getNodeConditionByType(clusterNode.Status.NodeStatus.Conditions, v1.NodeMemoryPressure).Status,
	}
	stats[clusterName][clusterNodeName] = newData
	nodeNameToClusterName[clusterNodeName] = clusterName
	s.aggregate(cluster, clusterName)
	err = s.update(cluster)
	if err != nil {
		stats[clusterName][clusterNodeName] = oldData
		return err
	}
	logrus.Infof("Successfully updated cluster [%s] stats", clusterName)
	return nil
}

func (s *StatsAggregator) aggregate(cluster *clusterv1.Cluster, clusterName string) {
	// capacity keys
	pods, mem, cpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// allocatable keys
	apods, amem, acpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// requested keys
	rpods, rmem, rcpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}
	// limited keys
	lpods, lmem, lcpu := resource.Quantity{}, resource.Quantity{}, resource.Quantity{}

	condDisk := v1.ConditionTrue
	condMem := v1.ConditionTrue

	for _, v := range stats[clusterName] {
		pods.Add(*v.Capacity.Pods())
		mem.Add(*v.Capacity.Memory())
		cpu.Add(*v.Capacity.Cpu())

		apods.Add(*v.Allocatable.Pods())
		amem.Add(*v.Allocatable.Memory())
		acpu.Add(*v.Allocatable.Cpu())

		rpods.Add(*v.Requested.Pods())
		rmem.Add(*v.Requested.Memory())
		rcpu.Add(*v.Requested.Cpu())

		lpods.Add(*v.Limits.Pods())
		lmem.Add(*v.Limits.Memory())
		lcpu.Add(*v.Limits.Cpu())

		if condDisk == v1.ConditionTrue && v.ConditionNoDiskPressureStatus == v1.ConditionTrue {
			condDisk = v1.ConditionFalse
		}

		if condMem == v1.ConditionTrue && v.ConditionNoMemoryPressureStatus == v1.ConditionTrue {
			condMem = v1.ConditionFalse
		}
	}

	cluster.Status.Capacity = v1.ResourceList{v1.ResourcePods: pods, v1.ResourceMemory: mem, v1.ResourceCPU: cpu}
	cluster.Status.Allocatable = v1.ResourceList{v1.ResourcePods: apods, v1.ResourceMemory: amem, v1.ResourceCPU: acpu}
	cluster.Status.Requested = v1.ResourceList{v1.ResourcePods: rpods, v1.ResourceMemory: rmem, v1.ResourceCPU: rcpu}
	cluster.Status.Limits = v1.ResourceList{v1.ResourcePods: lpods, v1.ResourceMemory: lmem, v1.ResourceCPU: lcpu}

	setConditionStatus(cluster, clusterv1.ClusterConditionNoDiskPressure, condDisk)
	setConditionStatus(cluster, clusterv1.ClusterConditionNoMemoryPressure, condMem)
}

func (s *StatsAggregator) update(cluster *clusterv1.Cluster) error {
	_, err := s.Clusters.Update(cluster)
	return err
}

func (s *StatsAggregator) getCluster(clusterName string) (*clusterv1.Cluster, error) {
	return s.Clusters.Get(clusterName, metav1.GetOptions{})
}

func mp(i interface{}, msg string) {
	ans, _ := json.Marshal(i)
	logrus.Infof(msg+"  %s", string(ans))
}

func getNodeConditionByType(conditions []v1.NodeCondition, conditionType v1.NodeConditionType) *v1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return &v1.NodeCondition{}
}

func setConditionStatus(cluster *clusterv1.Cluster, conditionType clusterv1.ClusterConditionType, status v1.ConditionStatus) {
	condition := getConditionByType(cluster, conditionType)
	now := time.Now().Format(time.RFC3339)
	if condition != nil {
		if condition.Status != status {
			condition.LastTransitionTime = now
		}
		condition.Status = status
		condition.LastUpdateTime = now
	} else {
		cluster.Status.Conditions = append(cluster.Status.Conditions,
			clusterv1.ClusterCondition{Type: conditionType,
				Status:             status,
				LastUpdateTime:     now,
				LastTransitionTime: now})
	}
}

func getConditionByType(cluster *clusterv1.Cluster, conditionType clusterv1.ClusterConditionType) *clusterv1.ClusterCondition {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}
