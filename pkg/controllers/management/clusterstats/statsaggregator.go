package clusterstats

import (
	"context"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/pkg/errors"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	nodeRoleControlPlane       = "node-role.kubernetes.io/controlplane"
	nodeRoleControlPlaneHyphen = "node-role.kubernetes.io/control-plane"
	nodeRoleETCD               = "node-role.kubernetes.io/etcd"

	// quietPeriod marks the minimum period between sync calls for the same Cluster
	quietPeriod = time.Second * 10
)

var numericReg = regexp.MustCompile("[^0-9]")

type StatsAggregator struct {
	NodesLister    v3.NodeLister
	Clusters       v3.ClusterInterface
	ClusterManager *clustermanager.Manager

	// lastReconcile is a map[string]time.Time storing the last execution time of the sync reconciler for every Cluster key
	lastReconcile sync.Map
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
	// This handler enqueues the corresponding mgmt.Cluster for every mgmt.Node reconciliation. Other handlers may also rely on this.
	// Nonetheless, in order to prevent unnecessary reconciliation when a cluster has many nodes, we are throttling (and deduplicating) the trigger.
	machinesClient.AddHandler(ctx, "cluster-stats", s.machineChanged)
}

func (s *StatsAggregator) sync(key string, cluster *apimgmtv3.Cluster) (runtime.Object, error) {
	if cluster == nil {
		s.lastReconcile.Delete(key)
		return nil, nil
	}

	if d := s.getMinimumWaitTime(key); d > 0 {
		// re-enqueue this controller after the quiet period
		s.Clusters.Controller().EnqueueAfter(cluster.Namespace, cluster.Name, d)
		return nil, nil
	}

	updated, err := s.aggregate(cluster)
	if err == nil {
		s.lastReconcile.Store(key, time.Now())
	}
	return updated, err
}

func (s *StatsAggregator) getMinimumWaitTime(key string) time.Duration {
	if v, ok := s.lastReconcile.Load(key); ok {
		lastReconcile := v.(time.Time)
		if wait := quietPeriod - time.Since(lastReconcile); wait > 0 {
			// only return positive values
			// the workqueue implementation accounts for negative or zero values, but let's play safe
			return wait
		}
	}
	return 0
}

func (s *StatsAggregator) aggregate(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	allMachines, err := s.NodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	workerCounts := make(map[string]int)
	var machines []*apimgmtv3.Node
	// only include worker nodes
	for _, m := range allMachines {
		// if none are set, then nodes syncer has not completed
		if !m.Spec.Worker && !m.Spec.ControlPlane && !m.Spec.Etcd {
			return nil, errors.Errorf("node role cannot be determined because node %s has not finished syncing. retrying", m.Status.NodeName)
		}
		if isTaintedNoExecuteNoSchedule(m) && !m.Spec.Worker {
			continue
		}
		if m.Spec.InternalNodeSpec.Unschedulable {
			continue
		}

		if os, ok := m.Status.NodeLabels["kubernetes.io/os"]; ok && m.Spec.Worker {
			workerCounts[os]++
		}

		machines = append(machines, m)
	}

	origStatus := cluster.Status.DeepCopy()
	cluster = cluster.DeepCopy()

	cluster.Status.NodeCount = len(allMachines)
	if c, ok := workerCounts["windows"]; ok {
		cluster.Status.WindowsWorkerCount = c
	}
	if c, ok := workerCounts["linux"]; ok {
		cluster.Status.LinuxWorkerCount = c
	}

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

		if apimgmtv3.ClusterConditionNoDiskPressure.IsTrue(machine) {
			condDisk = v1.ConditionFalse
		}
		if apimgmtv3.ClusterConditionNoMemoryPressure.IsTrue(machine) {
			condMem = v1.ConditionFalse
		}
	}

	cluster.Status.Capacity = v1.ResourceList{v1.ResourcePods: pods, v1.ResourceMemory: mem, v1.ResourceCPU: cpu}
	cluster.Status.Allocatable = v1.ResourceList{v1.ResourcePods: apods, v1.ResourceMemory: amem, v1.ResourceCPU: acpu}
	cluster.Status.Requested = v1.ResourceList{v1.ResourcePods: rpods, v1.ResourceMemory: rmem, v1.ResourceCPU: rcpu}
	cluster.Status.Limits = v1.ResourceList{v1.ResourcePods: lpods, v1.ResourceMemory: lmem, v1.ResourceCPU: lcpu}

	if condDisk == v1.ConditionTrue {
		apimgmtv3.ClusterConditionNoDiskPressure.True(cluster)
	} else {
		apimgmtv3.ClusterConditionNoDiskPressure.False(cluster)
	}
	if condMem == v1.ConditionTrue {
		apimgmtv3.ClusterConditionNoMemoryPressure.True(cluster)
	} else {
		apimgmtv3.ClusterConditionNoMemoryPressure.False(cluster)
	}

	s.updateVersion(cluster)

	// Remove "agent.cluster.cattle.io/upgraded-v1.22" annotation
	err = s.restartAgentDeployment(cluster)
	if err != nil {
		return nil, err
	}

	if statusChanged(origStatus, &cluster.Status) {
		// Use UpdateStatus to only update the status subresource - Update() ignores status changes.
		// Return nil to avoid norman's redundant Update() write-back.
		if _, err := s.Clusters.UpdateStatus(cluster); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *StatsAggregator) updateVersion(cluster *apimgmtv3.Cluster) {
	userContext, err := s.ClusterManager.UserContextNoControllersReconnecting(cluster.Name, false)
	if err != nil {
		return
	}

	callWithTimeout(func() {
		// This has the tendency to timeout
		version, err := userContext.K8sClient.Discovery().ServerVersion()
		if err != nil {
			return
		}

		if version != nil {
			// These fields can vary depending on the node serving the request
			// Since they are not consumed, we omit them from the status to avoid unnecessary flickering of the fields
			version.BuildDate, version.Platform = "", ""
		}

		if version == nil {
			return
		}

		currentVersionOk := cluster.Status.Version != nil
		versionOk := version != nil

		// if we don't have an older version recorded, the new one could not be queried, or they are both populated and different
		if currentVersionOk != versionOk || (currentVersionOk && *cluster.Status.Version != *version) {
			cluster.Status.Version = version
		}
	})
}

// restartAgentDeployment removes an annotation on the cluster agent deployment template
// which will force a restart of the deployment. This annotation was used to stop handlers running from when the
// cluster was running k8s 1.22, however this is applied to every newly created cluster causing unnecessary rollouts.
func (s *StatsAggregator) restartAgentDeployment(cluster *apimgmtv3.Cluster) error {
	userContext, err := s.ClusterManager.UserContextNoControllers(cluster.Name)
	if err != nil {
		return err
	}

	deployment, err := userContext.Apps.Deployments("cattle-system").Get("cattle-cluster-agent", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		return nil
	}

	// Add annotation to the agent deployment template in order to force a restart
	if deployment.Spec.Template.Annotations["agent.cluster.cattle.io/upgraded-v1.22"] != "" {
		logrus.Tracef("statsAggregator: updated cluster %s to v1.22, annotating agent deployment", cluster.Name)
		toUpdate := deployment.DeepCopy()
		delete(toUpdate.Spec.Template.Annotations, "agent.cluster.cattle.io/upgraded-v1.22")
		_, err = userContext.Apps.Deployments("cattle-system").Update(toUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}

func statusChanged(currentStatus, newStatus *apimgmtv3.ClusterStatus) bool {
	if !reflect.DeepEqual(currentStatus.Conditions, newStatus.Conditions) {
		return true
	}

	if currentStatus.LinuxWorkerCount != newStatus.LinuxWorkerCount ||
		currentStatus.WindowsWorkerCount != newStatus.WindowsWorkerCount ||
		currentStatus.NodeCount != newStatus.NodeCount {
		return true
	}

	if resourceListChanged(currentStatus.Capacity, newStatus.Capacity) {
		return true
	}

	if resourceListChanged(currentStatus.Allocatable, newStatus.Allocatable) {
		return true
	}

	if resourceListChanged(currentStatus.Requested, newStatus.Requested) {
		return true
	}

	if resourceListChanged(currentStatus.Limits, newStatus.Limits) {
		return true
	}

	currentVersionOk := currentStatus.Version != nil
	versionOk := newStatus.Version != nil

	// if we don't have an older version recorded, the new one could not be queried, or they are both populated and different
	if currentVersionOk != versionOk || (currentVersionOk && *currentStatus.Version != *newStatus.Version) {
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
		defer close(done)
		do()
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
	}
}

func (s *StatsAggregator) machineChanged(_ string, machine *apimgmtv3.Node) (runtime.Object, error) {
	if machine == nil {
		return nil, nil
	}

	if d := s.getMinimumWaitTime(machine.Namespace); d > 0 {
		s.Clusters.Controller().EnqueueAfter("", machine.Namespace, d)
	} else {
		// EnqueueAfter with a zero duration short-circuits to workqueue.Add, bypassing the rate-limiter
		s.Clusters.Controller().Enqueue("", machine.Namespace)
	}

	return nil, nil
}

func isTaintedNoExecuteNoSchedule(machine *apimgmtv3.Node) bool {
	for _, taint := range machine.Spec.InternalNodeSpec.Taints {
		isServer := taint.Key == nodeRoleControlPlane || taint.Key == nodeRoleETCD || taint.Key == nodeRoleControlPlaneHyphen
		if isServer && (taint.Effect == "NoSchedule" || taint.Effect == "NoExecute") {
			return true
		}
	}
	return false
}
