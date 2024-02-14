package k3sbasedupgrade

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	helmcfg "github.com/rancher/rancher/pkg/catalogv2/helm"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/steve/pkg/client"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/generic"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
)

const (
	// PSPAnswersField is passed to the helm --set command and denotes if we want to enable PodSecurityPolicies
	// when deploying the app, overriding the default value of 'true'.
	// In clusters >= 1.25 PSP's are not available, however we should
	// continue to deploy them in sub 1.25 clusters as they are required for cluster hardening.
	PSPAnswersField = "global.cattle.psp.enabled"
)

func (h *handler) onClusterChange(key string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}
	isK3s := cluster.Status.Driver == v32.ClusterDriverK3s
	isRke2 := cluster.Status.Driver == v32.ClusterDriverRke2
	// only applies to k3s/rke2 clusters
	if !isK3s && !isRke2 {
		return cluster, nil
	}
	// Don't allow nil configs to continue for given cluster type
	if (isK3s && cluster.Spec.K3sConfig == nil) || (isRke2 && cluster.Spec.Rke2Config == nil) {
		return cluster, nil
	}

	var (
		updateVersion string
		strategy      v32.ClusterUpgradeStrategy
	)
	switch {
	case isK3s:
		updateVersion = cluster.Spec.K3sConfig.Version
		strategy = cluster.Spec.K3sConfig.ClusterUpgradeStrategy
	case isRke2:
		updateVersion = cluster.Spec.Rke2Config.Version
		strategy = cluster.Spec.Rke2Config.ClusterUpgradeStrategy

	}
	if updateVersion == "" {
		return cluster, nil
	}

	// Check if the cluster is undergoing a Kubernetes version upgrade, and that
	// all downstream nodes also need the upgrade
	isNewer, err := IsNewerVersion(cluster.Status.Version.GitVersion, updateVersion)
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
			if v32.ClusterConditionUpgraded.IsUnknown(cluster) {
				v32.ClusterConditionUpgraded.True(cluster)
				v32.ClusterConditionUpgraded.Message(cluster, "")
				return h.clusterClient.Update(cluster)
			}
			return cluster, nil
		}

	}

	// set cluster upgrading status
	cluster, err = h.modifyClusterCondition(cluster, planv1.Plan{}, planv1.Plan{}, strategy)
	if err != nil {
		return cluster, err

	}

	// create or update k3supgradecontroller if necessary
	if err = h.deployK3sBasedUpgradeController(cluster.Name, updateVersion, isK3s, isRke2); err != nil {
		// TODO   Shouldn't an error here modify the cluster status from updating to erro?
		// Dosen't this reutrn make the cluster to get "stuck" on updating ?
		return cluster, err
	}

	if err = h.deployPlans(cluster, isK3s, isRke2); err != nil {
		return cluster, err
	}

	return cluster, nil
}

// deployK3sBaseUpgradeController creates a rancher k3s/rke2 upgrader controller if one does not exist.
// Updates k3s upgrader controller if one exists and is not the newest available version.
func (h *handler) deployK3sBasedUpgradeController(clusterName, updateVersion string, isK3s, isRke2 bool) error {

	userCtx, err := h.manager.UserContextNoControllers(clusterName)
	if err != nil {
		return err
	}

	// TODO _ CHECK HOW THIS WILL INTERACT WITH THE ONE FROM APPS, DO I NEED TO UNINSTALL? ???
	//   Can i just call delete and ignore the error if it is not found or all errors?
	_ = deleteOldApp(userCtx, clusterName, isK3s, isRke2)

	m, err := h.newDownstreamManagerFromUserContext(userCtx)
	if err != nil {
		return err
	}

	// determine what version of Kubernetes we are updating to
	is125OrAbove, err := Is125OrAbove(updateVersion)
	if err != nil {
		return err
	}

	//	TODO - I don't think it is possible to get the systemDefaultRegistry from the cluster, rancher gets it from Spec.RKEConfig witch
	//	 dosen't exist for imported ones.
	//	   The image.GetPrivateRepoURLFromCluster(cluster) will return the Default for clusters without spec.RKE witch is the case for imported clusters.
	value := map[string]interface{}{
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{
				"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
				"psp": map[string]interface{}{
					"enabled": !is125OrAbove,
				},
			},
		},
	}

	// Ensure won't install if the requested chart is already installed with the same args.
	// it will only return when the deployment is done or if an error happens.
	if err := m.Ensure("cattle-system", "system-upgrade-controller",
		"", settings.SystemUpgradeControllerChartVersion.Get(), value, true, ""); err != nil {
		log.Errorf("Failed to install the system-upgrade-controller with error: %s", err.Error())
		return err
	}

	return nil
}

// Is125OrAbove determines if a particular Kubernetes version is
// equal to or greater than 1.25.0
func Is125OrAbove(version string) (bool, error) {
	return IsNewerVersion("v1.24.99", version)
}

// IsNewerVersion returns true if updated versions semver is newer and false if its
// semver is older. If semver is equal then metadata is alphanumerically compared.
func IsNewerVersion(prevVersion, updatedVersion string) (bool, error) {
	parseErrMsg := "failed to parse version: %v"
	prevVer, err := semver.NewVersion(strings.TrimPrefix(prevVersion, "v"))
	if err != nil {
		return false, fmt.Errorf(parseErrMsg, err)
	}

	updatedVer, err := semver.NewVersion(strings.TrimPrefix(updatedVersion, "v"))
	if err != nil {
		return false, fmt.Errorf(parseErrMsg, err)
	}

	switch updatedVer.Compare(*prevVer) {
	case -1:
		return false, nil
	case 1:
		return true, nil
	default:
		// using metadata to determine precedence is against semver standards
		// this is ignored because it because k3s uses it to precedence between
		// two versions based on same k8s version
		return updatedVer.Metadata > prevVer.Metadata, nil
	}
}

// nodeNeedsUpgrade checks all nodes in cluster, returns true if they still need to be upgraded
func (h *handler) nodesNeedUpgrade(cluster *v3.Cluster, version string) (bool, error) {
	v3NodeList, err := h.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return false, err
	}
	for _, node := range v3NodeList {
		isNewer, err := IsNewerVersion(node.Status.InternalNodeStatus.NodeInfo.KubeletVersion, version)
		if err != nil {
			return false, err
		}
		if isNewer {
			return true, nil
		}
	}
	return false, nil
}

func checkDeployed(app *v33.App) bool {
	return v33.AppConditionDeployed.IsTrue(app) || v33.AppConditionInstalled.IsTrue(app)
}

// newDownstreamManagerFromUserContext will create a DownstreamManager "Handler" that interacts with the downstream cluster to install charts.
// As this will interact with the downstream cluster and will be created on every interaction of the control loop that needs it
// the Handler won't use cache, instead it will directly fetch the information of the downstream cluster.
func (h *handler) newDownstreamManagerFromUserContext(userCtx *config.UserContext) (*system.DownstreamManager, error) {

	helm, err := catalog.NewFactoryFromConfigWithOptions(&userCtx.RESTConfig,
		&generic.FactoryOptions{SharedControllerFactory: userCtx.ControllerFactory})
	if err != nil {
		return nil, err
	}

	cg, err := client.NewFactory(&userCtx.RESTConfig, false)
	if err != nil {
		return nil, err
	}

	content := content.NewManager(
		userCtx.K8sClient.Discovery(),
		system.ConfigMapNoOptGetter{ConfigMapClient: userCtx.Corew.ConfigMap()},
		system.SecretNoOptGetter{SecretClient: userCtx.Corew.Secret()},
		system.HelmNoNamespaceNoOptGetter{ClusterRepoController: helm.Catalog().V1().ClusterRepo()},
	)

	helmop := helmop.NewOperations(cg,
		helm.Catalog().V1(),
		userCtx.RBACw,
		content,
		userCtx.Corew.Pod(),
	)

	cache := memory.NewMemCacheClient(userCtx.K8sClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cache)

	restClientGetter := &wrangler.SimpleRESTClientGetter{
		ClientConfig:    nil, // The Manager don't use the ClientConfig. Therefore, we can pass this as nill.
		RESTConfig:      &userCtx.RESTConfig,
		CachedDiscovery: cache,
		RESTMapper:      restMapper,
	}
	helmClient := helmcfg.NewClient(restClientGetter)

	return system.NewDownstreamManager(h.ctx, content, helmop, userCtx.Corew.Pod(), helmClient)
}

// deleteOldApp will try to delete the v3.app rancher-xxx-upgrader that used to manage the system-upgrade-controller
func deleteOldApp(userCtx *config.UserContext, clusterName string, isK3s, isRke2 bool) error {
	projectLister := userCtx.Management.Management.Projects("").Controller().Lister()
	systemProject, err := project.GetSystemProject(clusterName, projectLister)
	if err != nil {
		return err
	}

	systemProjectID := ref.Ref(systemProject)
	_, systemProjectName := ref.Parse(systemProjectID)

	appClient := userCtx.Management.Project.Apps(systemProjectName)
	var appname string
	switch {
	case isK3s:
		appname = "rancher-k3s-upgrader"
	case isRke2:
		appname = "rancher-rke2-upgrader"
	}
	err = appClient.Delete(appname, &v1.DeleteOptions{})
	return err

}
