package clusterstats

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatsAggregator struct {
	Clusters v3.ClusterInterface
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

func (s *StatsAggregator) sync(key string, clusterNode *v3.Machine) error {
	logrus.Debugf("Syncing clusternode [%s]", key)
	if clusterNode == nil {
		return s.deleteStats(key)
	}
	return s.addOrUpdateStats(clusterNode)
}

func (s *StatsAggregator) deleteStats(key string) error {
	if _, exists := nodeNameToClusterName[key]; !exists {
		logrus.Debugf("ClusterNode [%s] already deleted from stats", key)
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
		logrus.Debugf("ClusterNode [%s] stats deleted", key)
	}
	s.aggregate(cluster, clusterName)
	err = s.update(cluster)
	if err != nil {
		stats[clusterName][clusterNodeName] = oldData
		return err
	}
	logrus.Debugf("Successfully updated cluster [%s] stats", clusterName)
	return nil
}

func (s *StatsAggregator) addOrUpdateStats(clusterNode *v3.Machine) error {
	clusterName, clusterNodeName := clusterNode.Status.ClusterName, clusterNode.Status.NodeName
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
	logrus.Debugf("Successfully updated cluster [%s] stats", clusterName)
	return nil
}

func (s *StatsAggregator) aggregate(cluster *v3.Cluster, clusterName string) {
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
		if v == nil {
			continue
		}

		if v.Capacity != nil {
			pods.Add(*v.Capacity.Pods())
			mem.Add(*v.Capacity.Memory())
			cpu.Add(*v.Capacity.Cpu())
		}

		if v.Allocatable != nil {
			apods.Add(*v.Allocatable.Pods())
			amem.Add(*v.Allocatable.Memory())
			acpu.Add(*v.Allocatable.Cpu())
		}

		if v.Requested != nil {
			rpods.Add(*v.Requested.Pods())
			rmem.Add(*v.Requested.Memory())
			rcpu.Add(*v.Requested.Cpu())
		}

		if v.Limits != nil {
			lpods.Add(*v.Limits.Pods())
			lmem.Add(*v.Limits.Memory())
			lcpu.Add(*v.Limits.Cpu())
		}

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
	if condDisk == v1.ConditionTrue {
		v3.ClusterConditionNoDiskPressure.True(cluster)
	} else {
		v3.ClusterConditionNoDiskPressure.False(cluster)
	}
	if condMem == v1.ConditionTrue {
		v3.ClusterConditionNoMemoryPressure.True(cluster)
	} else {
		v3.ClusterConditionNoMemoryPressure.False(cluster)
	}
}

func (s *StatsAggregator) update(cluster *v3.Cluster) error {
	_, err := s.Clusters.Update(cluster)
	return err
}

func (s *StatsAggregator) getCluster(clusterName string) (*v3.Cluster, error) {
	return s.Clusters.Get(clusterName, metav1.GetOptions{})
}

func getNodeConditionByType(conditions []v1.NodeCondition, conditionType v1.NodeConditionType) *v1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return &v1.NodeCondition{}
}
