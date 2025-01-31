package k3sbasedupgrade

import (
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	prjv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	app2 "github.com/rancher/rancher/pkg/app"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
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
			// if upgrade was in progress, make sure to set the state back
			if mgmtv3.ClusterConditionUpgraded.IsUnknown(cluster) {
				logrus.Infof("[k3s-based-upgrader] finished upgrading cluster [%s]", cluster.Name)
				mgmtv3.ClusterConditionUpgraded.True(cluster)
				mgmtv3.ClusterConditionUpgraded.Message(cluster, "")
				return h.clusterClient.Update(cluster)
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

	// create or update k3supgradecontroller if necessary
	if err = h.deployK3sBasedUpgradeController(cluster); err != nil {
		return cluster, err
	}

	// deploy plans into downstream cluster
	if err = h.deployPlans(cluster); err != nil {
		return cluster, err
	}

	return cluster, nil
}

// deployK3sBaseUpgradeController creates a rancher k3s/rke2 upgrader controller if one does not exist.
// Updates k3s upgrader controller if one exists and is not the newest available version.
func (h *handler) deployK3sBasedUpgradeController(cluster *mgmtv3.Cluster) error {
	userCtx, err := h.manager.UserContextNoControllers(cluster.Name)
	if err != nil {
		return err
	}

	projectLister := userCtx.Management.Management.Projects("").Controller().Lister()
	systemProject, err := project.GetSystemProject(cluster.Name, projectLister)
	if err != nil {
		return err
	}

	templateID := k3sUpgraderCatalogName
	template, err := h.templateLister.Get(namespace.GlobalNamespace, templateID)
	if err != nil {
		return err
	}

	latestTemplateVersion, err := h.catalogManager.LatestAvailableTemplateVersion(template, cluster.Name)
	if err != nil {
		return err
	}

	creator, err := h.systemAccountManager.GetSystemUser(cluster.Name)
	if err != nil {
		return err
	}
	systemProjectID := ref.Ref(systemProject)
	_, systemProjectName := ref.Parse(systemProjectID)

	nsClient := userCtx.Core.Namespaces("")
	appProjectName, err := app2.EnsureAppProjectName(nsClient, systemProjectName, cluster.Name, systemUpgradeNS, creator.Name)
	if err != nil {
		return err
	}

	appLister := userCtx.Management.Project.Apps("").Controller().Lister()
	appClient := userCtx.Management.Project.Apps("")

	latestVersionID := latestTemplateVersion.ExternalID
	var appName string
	switch {
	case cluster.Status.Driver == mgmtv3.ClusterDriverK3s:
		appName = K3sAppName
	case cluster.Status.Driver == mgmtv3.ClusterDriverRke2:
		appName = Rke2AppName
	}
	app, err := appLister.Get(systemProjectName, appName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		logrus.Infof("[k3s-based-upgrader] installing app [%s] in cluster [%s]", appName, cluster.Name)
		desiredApp := &prjv3.App{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: systemProjectName,
				Annotations: map[string]string{
					"field.cattle.io/creatorId": creator.Name,
				},
			},
			Spec: prjv3.AppSpec{
				Description:     "Upgrade controller for k3s based clusters",
				ExternalID:      latestVersionID,
				ProjectName:     appProjectName,
				TargetNamespace: systemUpgradeNS,
			},
		}

		// k3s upgrader doesn't exist yet, so it will need to be created
		if _, err = appClient.Create(desiredApp); err != nil {
			return err
		}
	} else {
		if !checkDeployed(app) {
			if !prjv3.AppConditionForceUpgrade.IsUnknown(app) {
				prjv3.AppConditionForceUpgrade.Unknown(app)
			}
			logrus.Warnln("force redeploying system-upgrade-controller")
			if _, err = appClient.Update(app); err != nil {
				return err
			}
		}

		// everything is up-to-date and are set up properly, no need to update.
		if app.Spec.ExternalID == latestVersionID {
			return nil
		}

		desiredApp := app.DeepCopy()
		desiredApp.Spec.ExternalID = latestVersionID
		logrus.Infof("[k3s-based-upgrader] updating app [%s] in cluster [%s]", appName, cluster.Name)
		// new version of k3s upgrade available, or the valuesYaml have changed, update app
		if _, err = appClient.Update(desiredApp); err != nil {
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

func checkDeployed(app *prjv3.App) bool {
	return prjv3.AppConditionDeployed.IsTrue(app) || prjv3.AppConditionInstalled.IsTrue(app)
}
