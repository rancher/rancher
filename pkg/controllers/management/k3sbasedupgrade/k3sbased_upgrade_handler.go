package k3sbasedupgrade

import (
	"context"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	planClientset "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/upgrade.cattle.io/v1"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	K3sAppName  = "rancher-k3s-upgrader"
	Rke2AppName = "rancher-rke2-upgrader"
)

func (h *handler) onClusterChange(_ string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}
	// only applies to imported RKE2/K3s cluster, as well as the local cluster if it is an RKE2/K3s cluster
	if cluster.Status.Driver != mgmtv3.ClusterDriverK3s && cluster.Status.Driver != mgmtv3.ClusterDriverRke2 {
		return cluster, nil
	}

	if !importedclusterversionmanagement.Enabled(cluster) {
		var masterPlan, workerPlan planv1.Plan

		clusterCtx, err := h.manager.UserContextNoControllersReconnecting(cluster.Name, false)
		if err != nil {
			return cluster, err
		}
		if err := healthsyncer.IsAPIUp(h.ctx, clusterCtx.K8sClient.CoreV1().Namespaces()); err != nil {
			// skip further work if the cluster's API is not reachable,
			// this usually happen during cattle-cluster-agent being redeployed
			logrus.Debugf("[k3s-based-upgrader] [%s] cluster API is not reachable, will try again", cluster.Name)
			h.clusterEnqueueAfter(cluster.Name, time.Second*5)
			return cluster, nil
		}
		planConfig, err := planClientset.NewForConfig(&clusterCtx.RESTConfig)
		if err != nil {
			return cluster, err
		}
		planClient := planConfig.Plans(systemUpgradeNS)
		planList, err := planClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return cluster, err
		}
		for _, plan := range planList.Items {
			switch plan.Name {
			case k3sMasterPlanName, rke2MasterPlanName:
				masterPlan = plan
			case k3sWorkerPlanName, rke2WorkerPlanName:
				workerPlan = plan
			}
		}

		// If the cluster is NOT undergoing an upgrade, remove the rancher-managed plans,
		// otherwise, update the cluster condition to reflect the upgrading progress
		if mgmtv3.ClusterConditionUpgraded.IsTrue(cluster) {
			if masterPlan.Name != "" {
				logrus.Debugf("[k3s-based-upgrader] removing master plan %s from cluster %s", masterPlan.Name, cluster.Name)
				if err := planClient.Delete(context.TODO(), masterPlan.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					return cluster, err
				}
			}
			if workerPlan.Name != "" {
				logrus.Debugf("[k3s-based-upgrader] removing worker plan %s from cluster %s", workerPlan.Name, cluster.Name)
				if err := planClient.Delete(context.TODO(), workerPlan.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					return cluster, err
				}
			}
			return cluster, nil
		} else {
			return h.modifyClusterCondition(cluster, masterPlan, workerPlan, mgmtv3.ClusterUpgradeStrategy{})
		}
	}

	// Reaching this point indicates that the cluster version management is enabled on the cluster
	var (
		updateVersion string
		strategy      mgmtv3.ClusterUpgradeStrategy
	)

	// only applies to imported k3s/rke2 clusters
	if cluster.Status.Driver == mgmtv3.ClusterDriverK3s {
		if cluster.Spec.K3sConfig == nil {
			return cluster, nil
		}
		updateVersion = cluster.Spec.K3sConfig.Version
		strategy = cluster.Spec.K3sConfig.ClusterUpgradeStrategy
	} else if cluster.Status.Driver == mgmtv3.ClusterDriverRke2 {
		if cluster.Spec.Rke2Config == nil {
			return cluster, nil
		}
		updateVersion = cluster.Spec.Rke2Config.Version
		strategy = cluster.Spec.Rke2Config.ClusterUpgradeStrategy
	} else {
		return cluster, nil
	}

	// no version set on imported cluster
	if updateVersion == "" {
		return cluster, nil
	}

	// Check if the cluster is undergoing a Kubernetes version upgrade, and that
	// all downstream nodes also need the upgrade
	isNewer, err := nodesyncer.IsNewerVersion(cluster.Status.Version.GitVersion, updateVersion)
	if err != nil {
		return cluster, err
	}
	if !isNewer {
		needsUpgrade, err := h.nodesNeedUpgrade(cluster, updateVersion)
		if err != nil {
			return cluster, err
		}
		if !needsUpgrade {
			// if upgrade was in progress, make sure to set the state back to true
			if mgmtv3.ClusterConditionUpgraded.IsUnknown(cluster) {
				logrus.Debug("[k3s-based-upgrader] updating the Upgraded condition to true")
				cluster = cluster.DeepCopy()
				cluster = upgradeDone(cluster)
				if cluster, err = h.clusterClient.Update(cluster); err != nil {
					return nil, err
				}
				logrus.Infof("[k3s-based-upgrader] finished upgrading cluster [%s]", cluster.Name)
			}
			return cluster, nil
		}
	}

	// Reaching this point indicates that an upgrade is required.
	if mgmtv3.ClusterConditionUpgraded.IsTrue(cluster) {
		logrus.Infof("[k3s-based-upgrader] upgrading cluster [%s] version from [%s] to [%s]",
			cluster.Name, cluster.Status.Version.GitVersion, updateVersion)
		if isNewer {
			logrus.Debugf("[k3s-based-upgrader] upgrading cluster [%s] because cluster version [%s] is newer than observed version [%s]",
				cluster.Name, updateVersion, cluster.Status.Version.GitVersion)
		} else {
			logrus.Debugf("[k3s-based-upgrader] upgrading cluster [%s] because cluster version [%s] is newer than observed node version",
				cluster.Name, updateVersion)
		}
	}

	// set cluster upgrading status
	cluster, err = h.modifyClusterCondition(cluster, planv1.Plan{}, planv1.Plan{}, strategy)
	if err != nil {
		return cluster, err
	}

	// deploy plans into downstream cluster
	if err = h.deployPlans(cluster); err != nil {
		return cluster, err
	}

	return cluster, nil
}

// nodeNeedsUpgrade checks all nodes in cluster, returns true if they still need to be upgraded
func (h *handler) nodesNeedUpgrade(cluster *mgmtv3.Cluster, version string) (bool, error) {
	v3NodeList, err := h.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return false, err
	}
	for _, node := range v3NodeList {
		// if node is windows, skip upgrade check
		if os, ok := node.Status.NodeLabels[corev1.LabelOSStable]; ok && os != "windows" {
			isNewer, err := nodesyncer.IsNewerVersion(node.Status.InternalNodeStatus.NodeInfo.KubeletVersion, version)
			if err != nil {
				return false, err
			}
			if isNewer {
				logrus.Debugf("[k3s-based-upgrader] cluster [%s] version [%s] is newer than observed node [%s] version [%s]",
					cluster.Name, version, node.Name, node.Status.InternalNodeStatus.NodeInfo.KubeletVersion)
				return true, nil
			}
		}
	}
	return false, nil
}
