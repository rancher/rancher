package rkeworkerupgrader

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/locker"
	nodeserver "github.com/rancher/rancher/pkg/rkenodeconfigserver"
	"github.com/rancher/rancher/pkg/systemaccount"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ignoreKey   = "user.cattle.io/upgrade-policy"
	ignoreValue = "prevent"
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
		logrus.Infof("checking cluster [%s] for worker nodes upgrade", cluster.Name)

		if ok, err := uh.toUpgradeCluster(cluster); err != nil {
			return nil, err
		} else if ok {
			if err := uh.upgradeCluster(cluster, key); err != nil {
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
		logrus.Infof("cluster [%s]: creating node plan for node [%s]", cluster.Name, node.Name)
		return uh.updateNodePlan(node, cluster)
	}

	if v3.ClusterConditionUpgraded.IsUnknown(cluster) {
		// if sync is for a node that was just updated, do nothing
		if !ignoreNode(node) {
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
		}

		logrus.Infof("cluster [%s] worker-upgrade: call upgrade to reconcile for node [%s]", cluster.Name, node.Name)
		if err := uh.upgradeCluster(cluster, node.Name); err != nil {
			return nil, err
		}
		return node, nil
	}

	// proceed only if node and cluster's versions mismatch
	if cluster.Status.NodeVersion == node.Status.AppliedNodeVersion {
		return node, nil
	}

	nodePlan, err := uh.getNodePlan(node, cluster)
	if err != nil {
		return nil, err
	}

	if node.Status.AppliedNodeVersion == 0 {
		// node never received appliedNodeVersion
		if planChangedForUpgrade(nodePlan, node.Status.NodePlan.Plan) || planChangedForUpdate(nodePlan, node.Status.NodePlan.Plan) {
			return uh.updateNodePlan(node, cluster)
		}
	} else {
		if planChangedForUpgrade(nodePlan, node.Status.NodePlan.Plan) {
			logrus.Infof("cluster [%s] worker-upgrade: plan changed for node [%s], call upgrade to reconcile cluster", cluster.Name, node.Name)
			if err := uh.upgradeCluster(cluster, node.Name); err != nil {
				return nil, err
			}
			return node, nil
		}
		if planChangedForUpdate(nodePlan, node.Status.NodePlan.Plan) {
			logrus.Infof("cluster [%s] worker-upgrade: plan changed for update [%s]", cluster.Name, node.Name)
			return uh.updateNodePlan(node, cluster)
		}
	}

	return node, nil
}

func (uh *upgradeHandler) updateNodePlan(node *v3.Node, cluster *v3.Cluster) (*v3.Node, error) {
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

	logrus.Infof("cluster [%s] worker-upgrade: updating node [%s] with plan [%v]", cluster.Name, node.Name, np.Version)

	updated, err := uh.nodes.Update(nodeCopy)
	if err != nil {
		return nil, fmt.Errorf("error updating node [%s] with plan %v", node.Name, err)
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

func (uh *upgradeHandler) upgradeCluster(cluster *v3.Cluster, nodeName string) error {
	clusterName := cluster.Name

	uh.clusterLock.Lock(clusterName)
	defer uh.clusterLock.Unlock(clusterName)

	if !v3.ClusterConditionUpgraded.IsUnknown(cluster) {
		clusterCopy := cluster.DeepCopy()
		v3.ClusterConditionUpgraded.Unknown(clusterCopy)
		v3.ClusterConditionUpgraded.Message(clusterCopy, "updating worker nodes")
		var err error
		cluster, err = uh.clusters.Update(clusterCopy)
		if err != nil {
			return err
		}
		logrus.Infof("cluster [%s] worker-upgrade: updated cluster for upgrading", clusterName)
	}

	nodes, err := uh.nodeLister.List(clusterName, labels.Everything())
	if err != nil {
		return err
	}

	toPrepareMap, toProcessMap, upgradedMap, notReadyMap, filtered, upgrading, done := uh.filterNodes(nodes, cluster.Status.NodeVersion)

	maxAllowed, err := calculateMaxUnavailable(cluster.Spec.RancherKubernetesEngineConfig.UpgradeStrategy.MaxUnavailableWorker, filtered)
	if err != nil {
		return err
	}

	logrus.Debugf("cluster [%s] worker-upgrade: workerNodeInfo: nodes %v maxAllowed %v upgrading %v notReady %v "+
		"toProcess %v toPrepare %v done %v", cluster.Name, filtered, maxAllowed, upgrading, len(notReadyMap), keys(toProcessMap), keys(toPrepareMap), keys(upgradedMap))

	if len(notReadyMap) > maxAllowed {
		return fmt.Errorf("cluster [%s] worker-upgrade: not enough nodes to upgrade: nodes %v notReady %v maxUnavailable %v", clusterName, filtered, keys(notReadyMap), maxAllowed)
	}

	if len(notReadyMap) > 0 {
		// nodes are already unavailable, update plan to reconcile
		for name, node := range notReadyMap {
			if node.Status.NodePlan.Version == cluster.Status.NodeVersion {
				continue
			}

			if err := uh.processNode(node, cluster, "reconciling"); err != nil {
				return err
			}

			logrus.Infof("cluster [%s] worker-upgrade: updated unavailable node [%s] to attempt upgrade", clusterName, name)
		}
	}

	unavailable := upgrading + len(notReadyMap)

	if unavailable > maxAllowed {
		return fmt.Errorf("cluster [%s] worker-upgrade: more than allowed nodes upgrading for cluster: unavailable %v maxUnavailable %v", clusterName, unavailable, maxAllowed)
	}

	for name, node := range upgradedMap {
		if v3.NodeConditionUpgraded.IsTrue(node) {
			continue
		}

		if err := uh.updateNodeActive(node); err != nil {
			return err
		}

		logrus.Infof("cluster [%s] worker-upgrade: updated node [%s] to uncordon", clusterName, name)
	}

	for _, node := range toProcessMap {
		if node.Status.NodePlan.Version == cluster.Status.NodeVersion {
			continue
		}

		if err := uh.processNode(node, cluster, "upgrading"); err != nil {
			return err
		}

		logrus.Infof("cluster [%s] worker-upgrade: updated node [%s] to upgrade", clusterName, node.Name)
	}

	toDrain := cluster.Spec.RancherKubernetesEngineConfig.UpgradeStrategy.Drain
	var nodeDrainInput *v3.NodeDrainInput
	state := "cordon"
	if toDrain {
		nodeDrainInput = cluster.Spec.RancherKubernetesEngineConfig.UpgradeStrategy.DrainInput
		state = "drain"
	}

	for _, node := range toPrepareMap {
		if unavailable == maxAllowed {
			break
		}
		unavailable++

		if err := uh.prepareNode(node, toDrain, nodeDrainInput); err != nil {
			return err
		}

		logrus.Infof("cluster [%s] worker-upgrade: updated node [%s] to %s", clusterName, node.Name, state)
	}

	if done == filtered {
		logrus.Debugf("cluster [%s] worker-upgrade: cluster is done upgrading, done %v len(nodes) %v", clusterName, done, filtered)
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

func (uh *upgradeHandler) toUpgradeCluster(cluster *v3.Cluster) (bool, error) {
	if v3.ClusterConditionUpgraded.IsUnknown(cluster) {
		return true, nil
	}

	nodes, err := uh.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return false, err
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

		nodePlan, err := uh.getNodePlan(node, cluster)
		if err != nil {
			return false, err
		}

		if planChangedForUpgrade(nodePlan, node.Status.NodePlan.Plan) {
			return true, nil
		}
	}

	return false, nil
}

func calculateMaxUnavailable(maxUnavailable string, nodes int) (int, error) {
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

func keys(m map[string]*v3.Node) []string {
	k := []string{}
	for key := range m {
		k = append(k, key)
	}
	return k
}
