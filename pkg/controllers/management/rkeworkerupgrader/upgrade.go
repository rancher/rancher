package rkeworkerupgrader

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/pkg/locker"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	nodeserver "github.com/rancher/rancher/pkg/rkenodeconfigserver"
	"github.com/rancher/rancher/pkg/systemaccount"
	rkedefaults "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

type upgradeHandler struct {
	clusters             v3.ClusterInterface
	nodes                v3.NodeInterface
	nodeLister           v3.NodeLister
	clusterLister        v3.ClusterLister
	lookup               *nodeserver.BundleLookup
	systemAccountManager *systemaccount.Manager
	serviceOptionsLister v3.RKEK8sServiceOptionLister
	serviceOptions       v3.RKEK8sServiceOptionInterface
	sysImagesLister      v3.RKEK8sSystemImageLister
	sysImages            v3.RKEK8sSystemImageInterface
	clusterLock          *locker.Locker
	ctx                  context.Context
}

func Register(ctx context.Context, mgmt *config.ManagementContext, scaledContext *config.ScaledContext) {

	uh := &upgradeHandler{
		clusters:             mgmt.Management.Clusters(""),
		clusterLister:        mgmt.Management.Clusters("").Controller().Lister(),
		nodes:                mgmt.Management.Nodes(""),
		nodeLister:           mgmt.Management.Nodes("").Controller().Lister(),
		lookup:               nodeserver.NewLookup(scaledContext.Core.Namespaces(""), scaledContext.Core),
		systemAccountManager: systemaccount.NewManagerFromScale(scaledContext),
		serviceOptionsLister: mgmt.Management.RKEK8sServiceOptions("").Controller().Lister(),
		serviceOptions:       mgmt.Management.RKEK8sServiceOptions(""),
		sysImagesLister:      mgmt.Management.RKEK8sSystemImages("").Controller().Lister(),
		sysImages:            mgmt.Management.RKEK8sSystemImages(""),
		clusterLock:          locker.New(),
		ctx:                  ctx,
	}

	mgmt.Management.Nodes("").Controller().AddHandler(ctx, "rke-worker-upgrader", uh.Sync)
}

func (uh *upgradeHandler) Sync(key string, node *v3.Node) (runtime.Object, error) {
	if strings.HasSuffix(key, "upgrade_") {

		cName := strings.Split(key, "/")[0]
		cluster, err := uh.clusterLister.Get("", cName)
		if err != nil {
			return nil, err
		}
		if cluster.DeletionTimestamp != nil || cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
			return nil, nil
		}

		if cluster.Annotations[clusterprovisioner.RkeRestoreAnnotation] == "true" {
			// rke completed etcd snapshot restore, need to update plan for nodes so worker nodes reset to old rkeConfig
			return node, uh.restore(cluster)
		}

		logrus.Infof("checking cluster [%s] for worker nodes upgrade", cluster.Name)

		if toUpgrade, planChanged, err := uh.toUpgradeCluster(cluster); err != nil {
			return nil, err
		} else if toUpgrade {
			if err := uh.upgradeCluster(cluster, key, planChanged); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}

	if node == nil || node.DeletionTimestamp != nil || !v3.NodeConditionProvisioned.IsTrue(node) {
		return node, nil
	}

	cluster, err := uh.clusterLister.Get("", node.Namespace)
	if err != nil {
		return nil, err
	}

	if cluster.DeletionTimestamp != nil || cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return node, nil
	}

	if node.Status.NodePlan == nil {
		return uh.updateNodePlan(node, cluster, true)
	}

	if v3.ClusterConditionUpdated.IsUnknown(cluster) || cluster.Annotations[clusterprovisioner.RkeRestoreAnnotation] == "true" {
		return node, nil
	}

	if v3.ClusterConditionUpgraded.IsUnknown(cluster) {
		// if sync is for a node that was just updated, do nothing

		// node is already updated to cordon/drain/uncordon, do nothing
		if node.Spec.DesiredNodeUnschedulable != "" {
			logrus.Debugf("cluster [%s] worker-upgrade: return node [%s], unschedulable field [%v]", cluster.Name,
				node.Name, node.Spec.DesiredNodeUnschedulable)
			return node, nil
		}
		// node is upgrading and already updated with new node plan, do nothing
		if v3.NodeConditionUpgraded.IsUnknown(node) && node.Status.AppliedNodeVersion != cluster.Status.NodeVersion {
			if node.Status.NodePlan.Version == cluster.Status.NodeVersion {
				logrus.Debugf("cluster [%s] worker-upgrade: return node [%s], plan's updated [%v]", cluster.Name,
					node.Name, cluster.Status.NodeVersion)
				return node, nil
			}
		}

		logrus.Infof("cluster [%s] worker-upgrade: call upgrade to reconcile for node [%s]", cluster.Name, node.Name)
		if err := uh.upgradeCluster(cluster, node.Name, false); err != nil {
			return nil, err
		}
		return node, nil
	}

	// proceed only if node and cluster's versions mismatch
	if cluster.Status.NodeVersion == node.Status.NodePlan.Version {
		return node, nil
	}

	nodePlan, err := uh.getNodePlan(node, cluster)
	if err != nil {
		return nil, err
	}

	if node.Status.AppliedNodeVersion == 0 {
		// node never received appliedNodeVersion
		if planChangedForUpgrade(nodePlan, node.Status.NodePlan.Plan) || planChangedForUpdate(nodePlan, node.Status.NodePlan.Plan) {
			return uh.updateNodePlan(node, cluster, false)
		}
	} else {
		if planChangedForUpdate(nodePlan, node.Status.NodePlan.Plan) {
			logrus.Infof("cluster [%s] worker-upgrade: plan changed for update [%s]", cluster.Name, node.Name)
			return uh.updateNodePlan(node, cluster, false)
		}
	}

	return node, nil
}

func (uh *upgradeHandler) updateNodePlan(node *v3.Node, cluster *v3.Cluster, create bool) (*v3.Node, error) {
	if node.Status.NodeConfig == nil || node.Status.DockerInfo == nil {
		logrus.Debugf("cluster [%s] worker-upgrade: node [%s] waiting for node status sync: "+
			"nodeConfigNil [%v] dockerInfoNil [%v]", cluster.Name, node.Name, node.Status.NodeConfig == nil, node.Status.DockerInfo == nil)
		// can't create correct node plan if node config or docker info hasn't been set
		return node, nil
	}
	nodePlan, err := uh.getNodePlan(node, cluster)
	if err != nil {
		return nil, fmt.Errorf("getNodePlan error for node [%s]: %v", node.Name, err)
	}

	nodeCopy := node.DeepCopy()
	np := &v3.NodePlan{
		Plan:    nodePlan,
		Version: cluster.Status.NodeVersion,
	}
	if node.Status.NodePlan == nil || node.Status.NodePlan.AgentCheckInterval == 0 {
		// default
		np.AgentCheckInterval = nodeserver.DefaultAgentCheckInterval
	} else {
		np.AgentCheckInterval = node.Status.NodePlan.AgentCheckInterval
	}
	nodeCopy.Status.NodePlan = np

	updated, err := uh.nodes.Update(nodeCopy)
	if err != nil {
		return nil, fmt.Errorf("error updating node [%s] with plan %v", node.Name, err)
	}

	if create {
		logrus.Debugf("cluster [%s]: created node plan for node [%s]", cluster.Name, node.Name)
	} else {
		logrus.Debugf("cluster [%s] worker-upgrade: updated node [%s] with plan [%v]", cluster.Name, node.Name, np.Version)
	}

	return updated, err
}

func (uh *upgradeHandler) updateNodePlanVersion(node *v3.Node, cluster *v3.Cluster) (*v3.Node, error) {
	nodeCopy := node.DeepCopy()
	nodeCopy.Status.NodePlan.Version = cluster.Status.NodeVersion
	logrus.Infof("cluster [%s] worker-upgrade: updating node [%s] with plan version [%v]", cluster.Name,
		node.Name, nodeCopy.Status.NodePlan.Version)

	updated, err := uh.nodes.Update(nodeCopy)
	if err != nil {
		return nil, err
	}
	return updated, err

}

func (uh *upgradeHandler) getNodePlan(node *v3.Node, cluster *v3.Cluster) (*v3.RKEConfigNodePlan, error) {
	var (
		nodePlan *v3.RKEConfigNodePlan
		err      error
	)
	if nodeserver.IsNonWorker(node.Status.NodeConfig.Role) {
		nodePlan, err = uh.nonWorkerPlan(node, cluster)
	} else {
		nodePlan, err = uh.workerPlan(node, cluster)
	}
	return nodePlan, err
}

func (uh *upgradeHandler) upgradeCluster(cluster *v3.Cluster, nodeName string, planChanged bool) error {
	clusterName := cluster.Name

	uh.clusterLock.Lock(clusterName)
	defer uh.clusterLock.Unlock(clusterName)

	logrus.Debugf("cluster [%s] upgrading condition: [%v] plan changed: [%v]", clusterName, v3.ClusterConditionUpgraded.IsUnknown(cluster), planChanged)

	var (
		clusterCopy *v3.Cluster
		err         error
	)
	if !v3.ClusterConditionUpgraded.IsUnknown(cluster) || planChanged {
		clusterCopy = cluster.DeepCopy()
		v3.ClusterConditionUpgraded.Unknown(clusterCopy)
		v3.ClusterConditionUpgraded.Message(clusterCopy, "updating worker nodes")
		clusterCopy.Status.NodeVersion++
	}
	if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.UpgradeStrategy == nil {
		if clusterCopy == nil {
			clusterCopy = cluster.DeepCopy()
		}
		clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.UpgradeStrategy = &v3.NodeUpgradeStrategy{
			MaxUnavailableWorker:       rkedefaults.DefaultMaxUnavailableWorker,
			MaxUnavailableControlplane: rkedefaults.DefaultMaxUnavailableControlplane,
			Drain:                      false,
		}
	}
	if clusterCopy != nil {
		cluster, err = uh.clusters.Update(clusterCopy)
		if err != nil {
			return err
		}
		logrus.Infof("cluster [%s] worker-upgrade: updated cluster nodeVersion [%v] upgradeStrategy [%v] ", clusterName,
			clusterCopy.Status.NodeVersion, cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.UpgradeStrategy)
	}

	logrus.Debugf("cluster [%s] worker-upgrade cluster status node version [%v]", clusterName, cluster.Status.NodeVersion)
	nodes, err := uh.nodeLister.List(clusterName, labels.Everything())
	if err != nil {
		return err
	}

	upgradeStrategy := cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.UpgradeStrategy
	toDrain := upgradeStrategy.Drain

	// get current upgrade status of nodes
	status := uh.filterNodes(nodes, cluster.Status.NodeVersion, toDrain)

	maxAllowed, err := CalculateMaxUnavailable(upgradeStrategy.MaxUnavailableWorker, status.filtered)
	if err != nil {
		return err
	}

	logrus.Debugf("cluster [%s] worker-upgrade: workerNodeInfo: nodes %v maxAllowed %v upgrading %v notReady %v "+
		"toProcess %v toPrepare %v done %v", cluster.Name, status.filtered, maxAllowed, status.upgrading,
		keys(status.notReady), keys(status.toProcess), keys(status.toPrepare), keys(status.upgraded))

	for _, node := range status.upgraded {
		if v3.NodeConditionUpgraded.IsTrue(node) {
			continue
		}

		if err := uh.updateNodeActive(node); err != nil {
			return err
		}

		logrus.Infof("cluster [%s] worker-upgrade: updated node [%s] to uncordon", clusterName, node.Name)
	}

	notReady := len(status.notReady)
	if notReady > maxAllowed {
		return fmt.Errorf("cluster [%s] worker-upgrade: not enough nodes to upgrade: nodes %v notReady %v maxUnavailable %v",
			clusterName, status.filtered, keys(status.notReady), maxAllowed)
	}

	if notReady > 0 {
		// update plan for unavailable nodes
		for _, node := range status.notReady {
			if node.Status.NodePlan.Version == cluster.Status.NodeVersion {
				continue
			}

			if err := uh.setNodePlan(node, cluster, false); err != nil {
				return err
			}

			logrus.Infof("cluster [%s] worker-upgrade: updated upgrading unavailable node [%s] with version %v",
				clusterName, node.Name, cluster.Status.NodeVersion)

		}
	}

	unavailable := status.upgrading + notReady

	if unavailable > maxAllowed {
		return fmt.Errorf("cluster [%s] worker-upgrade: more than allowed nodes upgrading for cluster: unavailable %v maxUnavailable %v",
			clusterName, unavailable, maxAllowed)
	}

	for _, node := range status.toProcess {
		if node.Status.NodePlan.Version == cluster.Status.NodeVersion {
			continue
		}

		if err := uh.setNodePlan(node, cluster, true); err != nil {
			return err
		}

		logrus.Infof("cluster [%s] worker-upgrade: updated node [%s] to upgrade", clusterName, node.Name)
	}

	var nodeDrainInput *v3.NodeDrainInput
	state := "cordon"
	if toDrain {
		nodeDrainInput = upgradeStrategy.DrainInput
		state = "drain"
	}

	for _, node := range status.toPrepare {
		if unavailable == maxAllowed {
			break
		}
		unavailable++

		if err := uh.prepareNode(node, toDrain, nodeDrainInput); err != nil {
			return err
		}

		logrus.Infof("cluster [%s] worker-upgrade: updated node [%s] to %s", clusterName, node.Name, state)
	}

	if status.done == status.filtered {
		logrus.Debugf("cluster [%s] worker-upgrade: cluster is done upgrading, done %v len(nodes) %v", clusterName, status.done, status.filtered)
		if !v3.ClusterConditionUpgraded.IsTrue(cluster) {
			clusterCopy := cluster.DeepCopy()
			v3.ClusterConditionUpgraded.True(clusterCopy)
			v3.ClusterConditionUpgraded.Message(clusterCopy, "")

			if _, err := uh.clusters.Update(clusterCopy); err != nil {
				return err
			}

			logrus.Infof("cluster [%s] worker-upgrade: finished upgrade", clusterName)
		}
	}

	return nil
}

func (uh *upgradeHandler) toUpgradeCluster(cluster *v3.Cluster) (bool, bool, error) {
	nodes, err := uh.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return false, false, err
	}

	for _, node := range nodes {
		if node.Status.NodeConfig == nil {
			continue
		}

		if !workerOnly(node.Status.NodeConfig.Role) {
			continue
		}

		if node.Status.NodePlan == nil {
			continue
		}

		// if cluster's already upgrading, skip nodes that are yet to upgrade, planChangedForUpgrade will always be true
		if v3.ClusterConditionUpgraded.IsUnknown(cluster) && node.Status.AppliedNodeVersion != cluster.Status.NodeVersion {
			continue
		}

		nodePlan, err := uh.getNodePlan(node, cluster)
		if err != nil {
			return false, false, err
		}

		if planChangedForUpgrade(nodePlan, node.Status.NodePlan.Plan) {
			return true, true, nil
		}
	}

	if v3.ClusterConditionUpgraded.IsUnknown(cluster) {
		return true, false, nil
	}

	return false, false, nil
}

func (uh *upgradeHandler) restore(cluster *v3.Cluster) error {
	nodes, err := uh.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}

	var errgrp errgroup.Group
	nodesQueue := util.GetObjectQueue(nodes)
	for w := 0; w < 5; w++ {
		errgrp.Go(func() error {
			var errList []error
			for node := range nodesQueue {
				node := node.(*v3.Node)
				if node.Status.NodeConfig != nil && workerOnly(node.Status.NodeConfig.Role) {
					if node.Status.NodePlan.Version != cluster.Status.NodeVersion {
						if err := uh.setNodePlan(node, cluster, false); err != nil {
							errList = append(errList, err)
						}
						logrus.Infof("cluster [%s]: updated node [%s] for restore, plan version [%v]", cluster.Name,
							node.Name, cluster.Status.NodeVersion)
					}
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}

	toUpdate := getRestoredCluster(cluster.DeepCopy())
	if _, err := uh.clusters.Update(toUpdate); err != nil {
		if !errors.IsConflict(err) {
			return err
		}
		return uh.retryClusterUpdate(cluster.Name)
	}
	return nil
}

func getRestoredCluster(cluster *v3.Cluster) *v3.Cluster {
	cluster.Annotations[clusterprovisioner.RkeRestoreAnnotation] = "false"

	// if restore's for a cluster stuck in upgrading, update it as upgraded
	if v3.ClusterConditionUpgraded.IsUnknown(cluster) {
		v3.ClusterConditionUpgraded.True(cluster)
		v3.ClusterConditionUpgraded.Message(cluster, "restored worker nodes")
	}

	return cluster
}

func (uh *upgradeHandler) retryClusterUpdate(name string) error {
	backoff := wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    7,
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		cluster, err := uh.clusters.Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if cluster.Annotations[clusterprovisioner.RkeRestoreAnnotation] != "false" {
			_, err = uh.clusters.Update(getRestoredCluster(cluster))
			if err != nil {
				logrus.Debugf("cluster [%s] restore: error resetting restore annotation %v", cluster.Name, err)
				if errors.IsConflict(err) {
					return false, nil
				}
				return false, err
			}
		}
		return true, nil
	})

}

func keys(nodes []*v3.Node) []string {
	keys := make([]string, len(nodes))
	for _, node := range nodes {
		keys = append(keys, node.Name)
	}
	return keys
}

func CalculateMaxUnavailable(maxUnavailable string, nodes int) (int, error) {
	parsedMax := intstr.Parse(maxUnavailable)
	maxAllowed, err := intstr.GetValueFromIntOrPercent(&parsedMax, nodes, false)
	if err != nil {
		return 0, err
	}
	if maxAllowed > 0 {
		return maxAllowed, nil
	}
	return 1, nil
}
