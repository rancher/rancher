package k3sbasedupgrade

import (
	"fmt"
	"sync/atomic"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	K3sAppName  = "rancher-k3s-upgrader"
	Rke2AppName = "rancher-rke2-upgrader"
)

// globalCounter keeps track of the number of clusters for which the handler is concurrently uninstalling the legacy K3s-based upgrade app.
// An atomic integer is used for efficiency, as it is lighter than a traditional lock.
var globalCounter atomic.Int32

func (h *handler) onClusterChange(_ string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

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

	if mgmtv3.ClusterConditionUpgraded.IsTrue(cluster) {
		if globalCounter.Load() < int32(settings.K3sBasedUpgraderUninstallConcurrency.GetInt()) {
			globalCounter.Add(1)
			err := h.uninstallK3sBasedUpgradeController(cluster)
			globalCounter.Add(-1)
			if err != nil {
				return nil, fmt.Errorf("[k3s-based-upgrader] failed to uninstall k3s based upgrade app: %w", err)
			}
		}
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
				mgmtv3.ClusterConditionUpgraded.True(cluster)
				mgmtv3.ClusterConditionUpgraded.Message(cluster, "")
				if cluster, err = h.clusterClient.Update(cluster); err != nil {
					return nil, err
				}
				logrus.Infof("[k3s-based-upgrader] finished upgrading cluster [%s]", cluster.Name)
			}
			return cluster, nil
		}
	}

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

// uninstallK3sBasedUpgradeController uninstalls the k3s-based-upgrader app from the cluster if it exists.
func (h *handler) uninstallK3sBasedUpgradeController(cluster *mgmtv3.Cluster) error {
	userCtx, err := h.manager.UserContextNoControllers(cluster.Name)
	if err != nil {
		return err
	}

	projectLister := userCtx.Management.Management.Projects("").Controller().Lister()
	systemProject, err := project.GetSystemProject(cluster.Name, projectLister)
	if err != nil {
		return err
	}

	appLister := userCtx.Management.Project.Apps("").Controller().Lister()
	appClient := userCtx.Management.Project.Apps("")

	var appName string
	switch {
	case cluster.Status.Driver == mgmtv3.ClusterDriverK3s:
		appName = K3sAppName
	case cluster.Status.Driver == mgmtv3.ClusterDriverRke2:
		appName = Rke2AppName
	}
	app, err := appLister.Get(systemProject.Name, appName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if app != nil && app.DeletionTimestamp == nil {
		logrus.Infof("[k3s-based-upgrader] uninstalling the app [%s] from the cluster [%s]", app.Name, cluster.Name)
		if err := appClient.DeleteNamespaced(app.Namespace, app.Name, &metav1.DeleteOptions{}); !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// nodeNeedsUpgrade checks all nodes in cluster, returns true if they still need to be upgraded
func (h *handler) nodesNeedUpgrade(cluster *mgmtv3.Cluster, version string) (bool, error) {
	v3NodeList, err := h.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return false, err
	}
	for _, node := range v3NodeList {
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
	return false, nil
}
