package clusterstats

import (
	"context"
	"reflect"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	nodeRoleControlPlane       = "node-role.kubernetes.io/controlplane"
	nodeRoleControlPlaneHyphen = "node-role.kubernetes.io/control-plane"
	nodeRoleETCD               = "node-role.kubernetes.io/etcd"
	nodeRoleMaster             = "node-role.kubernetes.io/master"
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

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	clustersClient := management.Management.Clusters("")
	machinesClient := management.Management.Nodes("")

	s := &StatsAggregator{
		NodesLister:    machinesClient.Controller().Lister(),
		Clusters:       clustersClient,
		ClusterManager: clusterManager,
	}

	clustersClient.AddHandler(ctx, "cluster-stats", s.sync)
	machinesClient.AddHandler(ctx, "cluster-stats", s.machineChanged)
}

func (s *StatsAggregator) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil {
		return nil, nil
	}

	return nil, s.aggregate(cluster, cluster.Name)
}

func (s *StatsAggregator) aggregate(cluster *v3.Cluster, clusterName string) error {
	allMachines, err := s.NodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}

	var machines []*v3.Node
	// only include worker nodes
	for _, m := range allMachines {
		// if none are set, then nodes syncer has not completed
		if !m.Spec.Worker && !m.Spec.ControlPlane && !m.Spec.Etcd {
			return errors.Errorf("node role cannot be determined because node %s has not finished syncing. retrying...", m.Status.NodeName)
		}
		if isTaintedNoExecuteNoSchedule(m) && !m.Spec.Worker {
			continue
		}
		if m.Spec.InternalNodeSpec.Unschedulable {
			continue
		}
		machines = append(machines, m)
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

		if condDisk == v1.ConditionTrue && v32.ClusterConditionNoDiskPressure.IsTrue(machine) {
			condDisk = v1.ConditionFalse
		}
		if condMem == v1.ConditionTrue && v32.ClusterConditionNoMemoryPressure.IsTrue(machine) {
			condMem = v1.ConditionFalse
		}
	}

	cluster.Status.Capacity = v1.ResourceList{v1.ResourcePods: pods, v1.ResourceMemory: mem, v1.ResourceCPU: cpu}
	cluster.Status.Allocatable = v1.ResourceList{v1.ResourcePods: apods, v1.ResourceMemory: amem, v1.ResourceCPU: acpu}
	cluster.Status.Requested = v1.ResourceList{v1.ResourcePods: rpods, v1.ResourceMemory: rmem, v1.ResourceCPU: rcpu}
	cluster.Status.Limits = v1.ResourceList{v1.ResourcePods: lpods, v1.ResourceMemory: lmem, v1.ResourceCPU: lcpu}
	if condDisk == v1.ConditionTrue {
		v32.ClusterConditionNoDiskPressure.True(cluster)
	} else {
		v32.ClusterConditionNoDiskPressure.False(cluster)
	}
	if condMem == v1.ConditionTrue {
		v32.ClusterConditionNoMemoryPressure.True(cluster)
	} else {
		v32.ClusterConditionNoMemoryPressure.False(cluster)
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
				isClusterVersionOk := cluster.Status.Version != nil
				isNewVersionOk := version != nil
				if isClusterVersionOk != isNewVersionOk ||
					(isClusterVersionOk && *cluster.Status.Version != *version) {
					cluster.Status.Version = version
					updated = true
				}
			}
		})
	}
	return updated
}

func statsChanged(existingCluster, newCluster *v32.ClusterStatus) bool {
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

func (s *StatsAggregator) machineChanged(key string, machine *v3.Node) (runtime.Object, error) {
	if machine != nil {
		s.Clusters.Controller().Enqueue("", machine.Namespace)
	}
	return nil, nil
}

func isTaintedNoExecuteNoSchedule(m *v3.Node) bool {
	for _, taint := range m.Spec.InternalNodeSpec.Taints {
		isETCDOrControlPlane := taint.Key == nodeRoleControlPlane || taint.Key == nodeRoleETCD ||
			taint.Key == nodeRoleControlPlaneHyphen || taint.Key == nodeRoleMaster
		if isETCDOrControlPlane && (taint.Effect == "NoSchedule" || taint.Effect == "NoExecute") {
			return true
		}
	}
	return false
}
