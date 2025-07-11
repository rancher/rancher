package clusterstats

import (
	"context"
	"reflect"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	agentVersionUpgraded       = "agent.cluster.cattle.io/upgraded-v1.22"

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
	machinesClient.AddHandler(ctx, "cluster-stats", s.machineChanged)
}

func (s *StatsAggregator) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil {
		s.lastReconcile.Delete(key)
		return nil, nil
	}

	if d := s.getMininumWaitTime(key); d > 0 {
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

func (s *StatsAggregator) getMininumWaitTime(key string) time.Duration {
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

func (s *StatsAggregator) aggregate(cluster *v3.Cluster) (*v3.Cluster, error) {
	allMachines, err := s.NodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	workerCounts := make(map[string]int)
	var machines []*v3.Node
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

		if v32.ClusterConditionNoDiskPressure.IsTrue(machine) {
			condDisk = v1.ConditionFalse
		}
		if v32.ClusterConditionNoMemoryPressure.IsTrue(machine) {
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

	var oldVersion int
	if cluster.Status.Version != nil {
		oldVersion, err = minorVersion(cluster)
		if err != nil {
			return nil, err
		}
	}
	versionChanged := s.updateVersion(cluster)

	// If the cluster went through an upgrade from <=1.21 to >=1.22, restart
	// the cluster agent in order to restart controllers that will no longer work
	// with the new API.
	if versionChanged && !cluster.Spec.Internal {
		newVersion, err := minorVersion(cluster)
		if err != nil {
			return nil, err
		}
		if newVersion >= 22 && oldVersion <= 21 {
			err := s.restartAgentDeployment(cluster)
			if err != nil {
				return nil, err
			}
		}
	}

	if statusChanged(origStatus, &cluster.Status) || versionChanged {
		return s.Clusters.Update(cluster)
	}
	return nil, nil
}

func minorVersion(cluster *v3.Cluster) (int, error) {
	minorVersion := numericReg.ReplaceAllString(cluster.Status.Version.Minor, "")
	return strconv.Atoi(minorVersion)
}

func (s *StatsAggregator) updateVersion(cluster *v3.Cluster) bool {
	updated := false
	userContext, err := s.ClusterManager.UserContextNoControllersReconnecting(cluster.Name, false)
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

// restartAgentDeployment sets an annotation on the cluster agent deployment template
// which will force a restart of the deployment. This is used when the cluster is
// upgraded to >=1.22 to ensure that controllers that are not compatible with v1.22 APIs
// are stopped, and controllers that are only compatible with v1.22 are started.
func (s *StatsAggregator) restartAgentDeployment(cluster *v3.Cluster) error {
	userContext, err := s.ClusterManager.UserContextNoControllers(cluster.Name)
	if err != nil {
		return err
	}
	deployment, err := userContext.Apps.Deployments("cattle-system").Get("cattle-cluster-agent", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	// Add annotation to the agent deployment template in order to force a restart
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	if deployment.Spec.Template.Annotations[agentVersionUpgraded] != "true" {
		logrus.Tracef("statsAggregator: updated cluster %s to v1.22, annotating agent deployment", cluster.Name)
		toUpdate := deployment.DeepCopy()
		toUpdate.Spec.Template.Annotations[agentVersionUpgraded] = "true"
		_, err = userContext.Apps.Deployments("cattle-system").Update(toUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}

func statusChanged(existingCluster, newCluster *v32.ClusterStatus) bool {
	if !reflect.DeepEqual(existingCluster.Conditions, newCluster.Conditions) {
		return true
	}

	if existingCluster.LinuxWorkerCount != newCluster.LinuxWorkerCount ||
		existingCluster.WindowsWorkerCount != newCluster.WindowsWorkerCount ||
		existingCluster.NodeCount != newCluster.NodeCount {
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
		defer close(done)
		do()
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
	}
}

func (s *StatsAggregator) machineChanged(key string, machine *v3.Node) (runtime.Object, error) {
	if machine == nil {
		return nil, nil
	}

	d := s.getMininumWaitTime(machine.Namespace)
	s.Clusters.Controller().EnqueueAfter("", machine.Namespace, d)

	return nil, nil
}

func isTaintedNoExecuteNoSchedule(m *v3.Node) bool {
	for _, taint := range m.Spec.InternalNodeSpec.Taints {
		isETCDOrControlPlane := taint.Key == nodeRoleControlPlane || taint.Key == nodeRoleETCD ||
			taint.Key == nodeRoleControlPlaneHyphen
		if isETCDOrControlPlane && (taint.Effect == "NoSchedule" || taint.Effect == "NoExecute") {
			return true
		}
	}
	return false
}
