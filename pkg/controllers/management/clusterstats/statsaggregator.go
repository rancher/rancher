package clusterstats

import (
	"reflect"
	"time"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

type StatsAggregator struct {
	NodesLister    v3.NodeLister
	Clusters       v3.ClusterInterface
	ClusterManager *clustermanager.Manager
}

type ClusterNodeData struct {
	Capacity                        v1.ResourceList
	Allocatable                     v1.ResourceList
	Requested                       v1.ResourceList
	Limits                          v1.ResourceList
	ConditionNoDiskPressureStatus   v1.ConditionStatus
	ConditionNoMemoryPressureStatus v1.ConditionStatus
}

func Register(management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	clustersClient := management.Management.Clusters("")
	machinesClient := management.Management.Nodes("")

	s := &StatsAggregator{
		NodesLister:    machinesClient.Controller().Lister(),
		Clusters:       clustersClient,
		ClusterManager: clusterManager,
	}

	clustersClient.AddHandler("cluster-stats", s.sync)
	machinesClient.AddHandler("cluster-stats", s.machineChanged)
}

func (s *StatsAggregator) sync(key string, cluster *v3.Cluster) error {
	if cluster == nil {
		return nil
	}

	return s.aggregate(cluster, cluster.Name)
}

func (s *StatsAggregator) aggregate(cluster *v3.Cluster, clusterName string) error {
	allMachines, err := s.NodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}

	var machines []*v3.Node
	// only include worker nodes
	for _, m := range allMachines {
		if m.Spec.Worker && !m.Spec.InternalNodeSpec.Unschedulable {
			machines = append(machines, m)
		}
	}

	origStatus := cluster.Status.DeepCopy()
	cluster = cluster.DeepCopy()

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

	for _, machine := range machines {
		capacity := machine.Status.InternalNodeStatus.Capacity
		if capacity != nil {
			pods.Add(*capacity.Pods())
			mem.Add(*capacity.Memory())
			cpu.Add(*capacity.Cpu())
		}
		allocatable := machine.Status.InternalNodeStatus.Allocatable
		if allocatable != nil {
			apods.Add(*allocatable.Pods())
			amem.Add(*allocatable.Memory())
			acpu.Add(*allocatable.Cpu())
		}
		requested := machine.Status.Requested
		if requested != nil {
			rpods.Add(*requested.Pods())
			rmem.Add(*requested.Memory())
			rcpu.Add(*requested.Cpu())
		}
		limits := machine.Status.Limits
		if limits != nil {
			lpods.Add(*limits.Pods())
			lmem.Add(*limits.Memory())
			lcpu.Add(*limits.Cpu())
		}

		if condDisk == v1.ConditionTrue && v3.ClusterConditionNoDiskPressure.IsTrue(machine) {
			condDisk = v1.ConditionFalse
		}
		if condMem == v1.ConditionTrue && v3.ClusterConditionNoMemoryPressure.IsTrue(machine) {
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

	versionChanged := s.updateVersion(cluster)

	if statsChanged(origStatus, &cluster.Status) || versionChanged {
		_, err = s.Clusters.Update(cluster)
		return err
	}

	return nil
}

func (s *StatsAggregator) updateVersion(cluster *v3.Cluster) bool {
	updated := false
	userContext, err := s.ClusterManager.UserContext(cluster.Name)
	if err == nil {
		callWithTimeout(func() {
			// This has the tendency to timeout
			version, err := userContext.K8sClient.Discovery().ServerVersion()
			if err == nil {
				if cluster.Status.Version != version {
					cluster.Status.Version = version
					updated = true
				}
			}
		})
	}
	return updated
}

func statsChanged(existingCluster, newCluster *v3.ClusterStatus) bool {
	if !reflect.DeepEqual(existingCluster.Conditions, newCluster.Conditions) {
		return true
	}

	if resourceListChanged(existingCluster.Capacity, newCluster.Capacity) {
		return true
	}

	if resourceListChanged(existingCluster.Allocatable, newCluster.Allocatable) {
		return true
	}

	if resourceListChanged(existingCluster.Requested, newCluster.Requested) {
		return true
	}

	if resourceListChanged(existingCluster.Limits, newCluster.Limits) {
		return true
	}

	return false
}

func resourceListChanged(oldList, newList v1.ResourceList) bool {
	if len(oldList) != len(newList) {
		return true
	}

	for k, v := range oldList {
		if v.Cmp(newList[k]) != 0 {
			return true
		}
	}

	return false
}

func callWithTimeout(do func()) {
	done := make(chan struct{})
	go func() {
		do()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
	}
}

func (s *StatsAggregator) machineChanged(key string, machine *v3.Node) error {
	if machine != nil {
		s.Clusters.Controller().Enqueue("", machine.Namespace)
	}
	return nil
}
