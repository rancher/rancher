// Package systemcharts handles the reconciliation of systemcharts installed by rancher in the rancher-charts repo.
package systemcharts

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/features"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	plancontrolers "github.com/rancher/rancher/pkg/generated/controllers/upgrade.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/data"
	admissionregcontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/admissionregistration.k8s.io/v1"
	deploymentControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	k8sappsv1 "k8s.io/api/apps/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	repoName             = "rancher-charts"
	sucDeploymentName    = "system-upgrade-controller"
	legacyAppFinalizer   = "systemcharts.cattle.io/legacy-k3s-based-upgrader-deprecation"
	managedPlanFinalizer = "systemcharts.cattle.io/rancher-managed-plan"

	// managedSucDeploymentAnno is added to the system-upgrade-controller chart since Rancher v2.12
	managedSucDeploymentAnno = "apps.cattle.io/managed-system-upgrade-controller"
)

var (
	primaryImages = map[string]string{
		chart.WebhookChartName:           "rancher/rancher-webhook",
		chart.RemoteDialerProxyChartName: "rancher/remotedialer-proxy",
		chart.TurtlesChartName:           "rancher/turtles",
	}
	// topLevelImagePullSecrets tracks the system charts that do not
	// use the standard 'global.cattle.imagePullSecrets' field to accept pull
	// secret references. Any chart listed in this map will have to subsequently
	// set up the image pull secrets in the chart specific Values function.
	topLevelImagePullSecrets = map[string]struct{}{
		chart.RemoteDialerProxyChartName: {},
		chart.TurtlesChartName:           {},
	}
	watchedSettings = map[string]struct{}{
		settings.RancherWebhookVersion.Name:               {},
		settings.RancherTurtlesVersion.Name:               {},
		settings.SystemDefaultRegistry.Name:               {},
		settings.ShellImage.Name:                          {},
		settings.SystemUpgradeControllerChartVersion.Name: {},
		settings.ImportedClusterVersionManagement.Name:    {},
		settings.SystemDefaultRegistryPullSecrets.Name:    {},
	}
	managedPlanSelector = labels.Set(map[string]string{k3sbasedupgrade.RancherManagedPlan: "true"}).AsSelector()
)

// Register is called to create a new handler and subscribe to change events.
func Register(ctx context.Context, wContext *wrangler.Context, registryOverride string) error {
	h := &handler{
		manager:                        wContext.SystemChartsManager,
		namespaces:                     wContext.Core.Namespace(),
		namespaceCache:                 wContext.Core.Namespace().Cache(),
		deployment:                     wContext.Apps.Deployment(),
		deploymentCache:                wContext.Apps.Deployment().Cache(),
		clusterRepo:                    wContext.Catalog.ClusterRepo(),
		clusterCache:                   wContext.Mgmt.Cluster().Cache(),
		plan:                           wContext.Upgrade.Plan(),
		planCache:                      wContext.Upgrade.Plan().Cache(),
		secrets:                        wContext.Core.Secret(),
		validatingWebhookConfiguration: wContext.Admission.ValidatingWebhookConfiguration(),
		mutatingWebhookConfigurations:  wContext.Admission.MutatingWebhookConfiguration(),
		chartsConfig:                   chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
		registryOverride:               registryOverride,
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-charts", h.onRepo)

	relatedresource.WatchClusterScoped(ctx, "bootstrap-charts", relatedFeatures, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Feature())

	relatedresource.WatchClusterScoped(ctx, "bootstrap-settings-charts", relatedSettings, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Setting())

	// ensure the system charts are installed with the correct values when there are changes to the rancher config map
	relatedresource.WatchClusterScoped(ctx, "bootstrap-configmap-charts", relatedConfigMaps, wContext.Catalog.ClusterRepo(), wContext.Core.ConfigMap())

	wContext.Apps.Deployment().OnChange(ctx, "legacy-k3sBasedUpgrader-deprecation", h.onDeployment)

	wContext.Upgrade.Plan().OnChange(ctx, "monitor-plans", h.onPlan)

	wContext.Mgmt.Cluster().OnChange(ctx, "monitor-local-cluster", h.onCluster)

	// TODO: remove in 2.15 - cleanup embedded CAPI webhooks when the namespace is deleted
	wContext.Core.Namespace().OnChange(ctx, "cleanup-embedded-capi-webhook-configs", h.removeCAPIWebhooks)

	return nil
}

type handler struct {
	manager                        chart.Manager
	namespaces                     corecontrollers.NamespaceController
	namespaceCache                 corecontrollers.NamespaceCache
	deployment                     deploymentControllers.DeploymentController
	deploymentCache                deploymentControllers.DeploymentCache
	clusterRepo                    catalogcontrollers.ClusterRepoController
	secrets                        corecontrollers.SecretController
	mutatingWebhookConfigurations  admissionregcontrollers.MutatingWebhookConfigurationController
	validatingWebhookConfiguration admissionregcontrollers.ValidatingWebhookConfigurationController
	chartsConfig                   chart.RancherConfigGetter
	clusterCache                   mgmtcontrollers.ClusterCache
	plan                           plancontrolers.PlanController
	planCache                      plancontrolers.PlanCache
	registryOverride               string
}

func (h *handler) onRepo(_ string, repo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	if repo == nil || repo.Name != repoName {
		return repo, nil
	}

	systemDefaultRegistry := settings.SystemDefaultRegistry.Get()
	var helmOpInstallImageOverride string
	if h.registryOverride != "" {
		// if we have a specific image override, don't set the system default registry. This will be the case when
		// these controllers are running in the downstream cattle-cluster-agent.
		systemDefaultRegistry = ""
		logrus.Tracef("[system-charts] registryOverride=%q set; systemDefaultRegistry cleared for per-chart image override", h.registryOverride)
		helmOpInstallImageOverride = h.registryOverride + "/" + settings.ShellImage.Get()
		logrus.Tracef("[system-charts] helmOpInstallImageOverride=%q", helmOpInstallImageOverride)
	}

	chartsToInstall := h.getChartsToInstall()
	logrus.Debugf("[system-charts] evaluating %d chart definition(s)", len(chartsToInstall))

	for _, chartDef := range chartsToInstall {
		logrus.Tracef("[system-charts] processing chart %q (namespace=%q, release=%q, uninstall=%v)", chartDef.ChartName, chartDef.ReleaseNamespace, chartDef.ReleaseName, chartDef.Uninstall)

		if chartDef.Uninstall {
			logrus.Debugf("[system-charts] uninstalling chart %q (removeNamespace=%v)", chartDef.ChartName, chartDef.RemoveNamespace)
			// it is important to remove the chart from the desired chart list
			h.manager.Remove(chartDef.ReleaseNamespace, chartDef.ChartName)
			if err := h.manager.Uninstall(chartDef.ReleaseNamespace, chartDef.ChartName); err != nil {
				logrus.Errorf("[system-charts] failed to uninstall chart %q: %v", chartDef.ChartName, err)
				return repo, err
			}
			if chartDef.RemoveNamespace {
				logrus.Debugf("[system-charts] deleting namespace %q for chart %q", chartDef.ReleaseNamespace, chartDef.ChartName)
				if err := h.namespaces.Delete(chartDef.ReleaseNamespace, nil); err != nil && !errors.IsNotFound(err) {
					logrus.Errorf("[system-charts] failed to delete namespace %q: %v", chartDef.ReleaseNamespace, err)
					return repo, err
				}
			}
			continue
		}

		if chartDef.Enabled != nil && !chartDef.Enabled() {
			logrus.Tracef("[system-charts] chart %q is disabled, skipping", chartDef.ChartName)
			continue
		}

		values := map[string]interface{}{
			"global": map[string]interface{}{
				"cattle": map[string]interface{}{
					"systemDefaultRegistry": systemDefaultRegistry,
				},
			},
		}

		if _, ok := topLevelImagePullSecrets[chartDef.ChartName]; !ok {
			global := values["global"].(map[string]interface{})
			cattle := global["cattle"].(map[string]interface{})
			h.setImagePullSecrets(cattle, false)
		}

		if image, ok := primaryImages[chartDef.ChartName]; ok {
			imageSettings := map[string]interface{}{}
			if h.registryOverride != "" {
				imageSettings["repository"] = h.registryOverride + "/" + image
			} else {
				// If we do not have a registry override, ensure we are passing the plain repository and image name.
				// Due to how the chart patching logic works, this is required to properly handle the case where a
				// private registry is no longer configured at the cluster or global level. If we do not explicitly
				// define this field, the old value (which points to the previously defined private registry) will always
				// be used.
				imageSettings["repository"] = image
			}
			logrus.Debugf("[system-charts] overriding image repository for chart %q to %q", chartDef.ChartName, imageSettings["repository"])
			values["image"] = imageSettings
		}

		if chartDef.Values != nil {
			chartSpecificValues := chartDef.Values()
			logrus.Tracef("[system-charts] merging %d chart-specific value(s) for chart %q", len(chartSpecificValues), chartDef.ChartName)
			for k, v := range chartSpecificValues {
				values[k] = v
			}
		}

		// webhook needs to be able to adopt the MutatingWebhookConfiguration which originally wasn't a part of the
		// chart definition, but is now part of the chart definition
		minVersion := chartDef.MinVersionSetting.Get()
		exactVersion := chartDef.ExactVersionSetting.Get()
		takeOwnership := chartDef.ChartName == chart.WebhookChartName
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, chartDef.ReleaseName, minVersion, exactVersion, values, takeOwnership, helmOpInstallImageOverride); err != nil {
			logrus.Errorf("[system-charts] failed to ensure chart %q: %v", chartDef.ChartName, err)
			return repo, err
		}

		logrus.Debugf("[system-charts] ensured chart %q in namespace %q", chartDef.ChartName, chartDef.ReleaseNamespace)
	}

	logrus.Tracef("[system-charts] reconciliation complete for repo %q", repoName)
	return repo, nil
}

func (h *handler) getChartsToInstall() []*chart.Definition {
	return []*chart.Definition{
		{
			ReleaseNamespace:    namespace.System,
			ReleaseName:         chart.RemoteDialerProxyChartName,
			ChartName:           chart.RemoteDialerProxyChartName,
			ExactVersionSetting: settings.RemoteDialerProxyVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{}
				// add priority class value
				h.setPriorityClass(values, chart.RemoteDialerProxyChartName)
				// add image pull secrets, RDP uses local object references at
				// the top level.
				h.setImagePullSecrets(values, true)
				return values
			},
			Enabled: func() bool {
				// do not deploy RDP in downstream cluster
				if features.MCMAgent.Enabled() {
					return false
				}

				return ext.RDPEnabled()
			},
			RemoveNamespace: false,
		},
		{
			ReleaseNamespace:    namespace.System,
			ReleaseName:         chart.WebhookChartName,
			ChartName:           chart.WebhookChartName,
			ExactVersionSetting: settings.RancherWebhookVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{
					// This is no longer used in the webhook chart but previous values can still be found
					// with `helm get values -n cattle-system rancher-webhook` which can be confusing. We
					// completely remove the previous capi values by setting it to nil here.
					"capi": nil,
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
				}
				// add priority class value
				h.setPriorityClass(values, chart.WebhookChartName)
				// get custom values for the rancher-webhook
				configMapValues := h.getChartValues(chart.WebhookChartName)
				return data.MergeMaps(values, configMapValues)
			},
			Enabled: func() bool { return true },
		},
		{
			ReleaseNamespace: "rancher-operator-system",
			ReleaseName:      "rancher-operator",
			ChartName:        "rancher-operator",
			Uninstall:        true,
			RemoveNamespace:  true,
		},
		{
			// TODO: remove in 2.15
			ReleaseNamespace: "cattle-provisioning-capi-system",
			ReleaseName:      "rancher-provisioning-capi",
			ChartName:        "rancher-provisioning-capi",
			Uninstall:        true,
			RemoveNamespace:  true,
		},
		{
			ReleaseNamespace:    namespace.TurtlesNamespace,
			ReleaseName:         chart.TurtlesChartName,
			ChartName:           chart.TurtlesChartName,
			ExactVersionSetting: settings.RancherTurtlesVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
				}
				// add image pull secrets, turtles uses string references at
				// the top level.
				h.setImagePullSecrets(values, false)
				// add priority class value
				h.setPriorityClass(values, chart.TurtlesChartName)
				// get custom values for rancher-turtles
				configMapValues := h.getChartValues(chart.TurtlesChartName)
				return data.MergeMaps(values, configMapValues)
			},
			Enabled: func() bool {
				return features.Turtles.Enabled()
			},
			Uninstall:       !features.Turtles.Enabled(),
			RemoveNamespace: !features.Turtles.Enabled(),
		},
		{
			ReleaseNamespace: namespace.System,
			ReleaseName: func() string {
				if name := os.Getenv("CATTLE_SUC_APP_NAME_OVERRIDE"); name != "" {
					return name
				}
				if isInHarvesterLocal() {
					return "mcc-local-managed-system-upgrade-controller"
				}
				return chart.SystemUpgradeControllerChartName
			}(),
			ChartName:           chart.SystemUpgradeControllerChartName,
			ExactVersionSetting: settings.SystemUpgradeControllerChartVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{}
				// add priority class value
				h.setPriorityClass(values, chart.SystemUpgradeControllerChartName)

				// override the image registries because the structure of the values
				// in this chart differs from the expected structure. Ensure that
				// we always set the image repository so that the current private
				// registry configuration is always respected.
				sucRepository := "rancher/system-upgrade-controller"
				kubectlRepository := "rancher/kuberlr-kubectl"
				if h.registryOverride != "" {
					sucRepository = fmt.Sprintf("%s/%s", h.registryOverride, "rancher/system-upgrade-controller")
					kubectlRepository = fmt.Sprintf("%s/%s", h.registryOverride, "rancher/kuberlr-kubectl")
				}
				values["systemUpgradeController"] = map[string]interface{}{
					"image": map[string]interface{}{
						"repository": sucRepository,
					},
				}
				values["kubectl"] = map[string]interface{}{
					"image": map[string]interface{}{
						"repository": kubectlRepository,
					},
				}

				// get custom values for system-upgrade-controller
				configMapValues := h.getChartValues(chart.SystemUpgradeControllerChartName)
				return data.MergeMaps(values, configMapValues)
			},
			Enabled: func() bool {
				toEnable := true
				suc, err := h.deploymentCache.Get(namespace.System, sucDeploymentName)
				if err != nil && !errors.IsNotFound(err) {
					toEnable = false
					logrus.Warnf("[system-charts] failed to get deployment %q/%q: %v", namespace.System, sucDeploymentName, err)
				}
				if suc != nil {
					// The missing annotation suggests that either the legacy Fleet bundle in the node-driver RKE2/K3s cluster,
					// or the old rancher-k3s/rke2-upgrader Project App in the imported RKE2/K3s cluster, is still present.
					// Note: these legacy components are cleaned up by other handlers within Rancher.
					if _, ok := suc.Annotations[managedSucDeploymentAnno]; !ok {
						toEnable = false
					}
				}
				var versionManagementEnabled bool
				if features.MCMAgent.Enabled() {
					// For the imported or node-driver/custom RKE2/K3s cluster,
					// cluster agent checks the ManagedSystemUpgradeController feature in the cluster
					versionManagementEnabled = features.ManagedSystemUpgradeController.Enabled()
				}
				if features.MCM.Enabled() {
					// for the local cluster,
					// Rancher has direct access to the mgmt v3 cluster and the ImportedClusterVersionManagement setting
					cluster, err := h.clusterCache.Get("local")
					if err != nil {
						logrus.Warnf("[system-charts] failed to get local cluster: %v", err)
					}
					if cluster != nil && (cluster.Status.Driver == v3.ClusterDriverRke2 || cluster.Status.Driver == v3.ClusterDriverK3s) {
						versionManagementEnabled = importedclusterversionmanagement.Enabled(cluster)
					}
				}
				if isInHarvesterLocal() {
					cluster, err := h.clusterCache.Get("local")
					if err != nil {
						logrus.Warnf("[system-charts] failed to get local cluster: %v", err)
					}
					if cluster != nil && cluster.Status.Provider == "harvester" && cluster.Status.Driver == v3.ClusterDriverImported {
						versionManagementEnabled = importedclusterversionmanagement.Enabled(cluster)
					}
				}
				toInstall := versionManagementEnabled && toEnable
				logrus.Debugf("[system-charts] install system-upgrade-controller: %v (versionManagementEnabled=%v, toEnable=%v)",
					toInstall, versionManagementEnabled, toEnable)
				return toInstall
			},
			Uninstall: func() bool {
				noManagedPlan := false
				// The absence of rancher-managed plans indicates the cluster is ready for uninstalling the app.
				// The removal of the plans is handled in the k3sbasedupgrade package.
				plans, err := h.planCache.List(namespace.System, managedPlanSelector)
				if err != nil {
					logrus.Warnf("[system-charts] failed to list plans: %v", err)
				}
				if len(plans) == 0 {
					noManagedPlan = true
				}

				var versionManagementEnabled bool
				if features.MCMAgent.Enabled() {
					// For the imported or node-driver/custom RKE2/K3s cluster,
					// cluster agent checks the ManagedSystemUpgradeController feature in the cluster
					versionManagementEnabled = features.ManagedSystemUpgradeController.Enabled()
				}
				if features.MCM.Enabled() {
					// for the local cluster,
					// Rancher has direct access to the mgmt v3 cluster and the ImportedClusterVersionManagement setting
					cluster, err := h.clusterCache.Get("local")
					if err != nil {
						logrus.Warnf("[system-charts] failed to get local cluster: %v", err)
					}
					if cluster != nil && (cluster.Status.Driver == v3.ClusterDriverRke2 || cluster.Status.Driver == v3.ClusterDriverK3s) {
						versionManagementEnabled = importedclusterversionmanagement.Enabled(cluster)
					}
				}
				if isInHarvesterLocal() {
					cluster, err := h.clusterCache.Get("local")
					if err != nil {
						logrus.Warnf("[system-charts] failed to get local cluster: %v", err)
					}
					if cluster != nil && cluster.Status.Provider == "harvester" && cluster.Status.Driver == v3.ClusterDriverImported {
						versionManagementEnabled = importedclusterversionmanagement.Enabled(cluster)
					}
				}
				toUninstall := !versionManagementEnabled && noManagedPlan
				logrus.Debugf("[system-charts] uninstall system-upgrade-controller: %v (versionManagementEnabled=%v, noManagedPlan=%v)",
					toUninstall, versionManagementEnabled, noManagedPlan)
				return toUninstall
			}(),
		},
	}
}

// onDeployment enqueues the rancher-charts ClusterRepo into the controller's processing queue in response to specific
// deployment events. It is currently used to handle the migration from the legacy k3s-based-upgrader app in
// imported clusters, and the Fleet-managed system-upgrade-controller in node-driver clusters,
// to the system-upgrade-controller app as a systemChart.
func (h *handler) onDeployment(_ string, d *k8sappsv1.Deployment) (*k8sappsv1.Deployment, error) {
	if d == nil || d.Namespace != namespace.System || d.Name != sucDeploymentName {
		return d, nil
	}
	if _, ok := d.Annotations[managedSucDeploymentAnno]; ok {
		return d, nil
	}

	index := slices.Index(d.Finalizers, legacyAppFinalizer)
	logrus.Tracef("[system-charts] deployment %q/%q: legacy finalizer index=%d", d.Namespace, d.Name, index)
	if (d.DeletionTimestamp != nil && index == -1) || (d.DeletionTimestamp == nil && index >= 0) {
		return d, nil
	}
	var err error
	switch {
	case d.DeletionTimestamp != nil && index >= 0:
		// When the deployment is being deleted, remove the finalizer if it exists, and enqueue the rancher-charts clusterRepo
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			d, err = h.deployment.Get(d.Namespace, d.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
			index := slices.Index(d.Finalizers, legacyAppFinalizer)
			if index == -1 {
				return nil
			}
			d = d.DeepCopy()
			d.Finalizers = append(d.Finalizers[:index], d.Finalizers[index+1:]...)
			d, err = h.deployment.Update(d)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to update deployment %s/%s: %w", d.Namespace, d.Name, err)
		}
		logrus.Debugf("[system-charts] enqueuing %q to reconcile after legacy SUC deployment cleanup", repoName)
		h.clusterRepo.EnqueueAfter(repoName, 2*time.Second)

	case d.DeletionTimestamp == nil && index == -1:
		// If the deployment is not being deleted, add the finalizer if it is absent to ensure Wrangler can detect the deletion event
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			d, err = h.deployment.Get(d.Namespace, d.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
			index := slices.Index(d.Finalizers, legacyAppFinalizer)
			if index >= 0 {
				return nil
			}
			d = d.DeepCopy()
			d.Finalizers = append(d.Finalizers, legacyAppFinalizer)
			d, err = h.deployment.Update(d)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to update deployment %s/%s: %w", d.Namespace, d.Name, err)
		}
	default:
		return d, nil
	}
	return d, nil
}

// onPlan manages the finalizers on rancher-managed Plan,
// it enqueues the rancher-charts ClusterRepo to the controller's processing queue when the Plan is deleted.
func (h *handler) onPlan(_ string, plan *upgradev1.Plan) (*upgradev1.Plan, error) {
	if plan == nil || plan.Namespace != namespace.System || plan.Labels[k3sbasedupgrade.RancherManagedPlan] != "true" {
		return plan, nil
	}
	index := slices.Index(plan.Finalizers, managedPlanFinalizer)
	logrus.Tracef("[system-charts] plan %q/%q: managed-plan finalizer index=%d", plan.Namespace, plan.Name, index)
	if (plan.DeletionTimestamp != nil && index == -1) || (plan.DeletionTimestamp == nil && index >= 0) {
		return plan, nil
	}
	var err error
	if plan.DeletionTimestamp != nil && index >= 0 {
		// When the plan is being deleted, remove the finalizer if it exists, and enqueue the rancher-charts clusterRepo
		plan, err = h.plan.Get(plan.Namespace, plan.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if plan == nil {
			return plan, nil
		}
		if index = slices.Index(plan.Finalizers, managedPlanFinalizer); index == -1 {
			return plan, nil
		}
		plan = plan.DeepCopy()
		plan.Finalizers = append(plan.Finalizers[:index], plan.Finalizers[index+1:]...)
		plan, err = h.plan.Update(plan)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("[system-charts] enqueuing %q to reconcile after managed plan deletion", repoName)
		h.clusterRepo.EnqueueAfter(repoName, 2*time.Second)
	}
	if plan.DeletionTimestamp == nil && index == -1 {
		// If the plan is not being deleted, add the finalizer if it is absent to ensure Wrangler can detect the deletion event
		plan, err = h.plan.Get(plan.Namespace, plan.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if plan == nil {
			return plan, nil
		}
		if index = slices.Index(plan.Finalizers, managedPlanFinalizer); index >= 0 {
			return plan, nil
		}
		plan = plan.DeepCopy()
		plan.Finalizers = append(plan.Finalizers, managedPlanFinalizer)
		plan, err = h.plan.Update(plan)
		if err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// onCluster enqueues the rancher-charts ClusterRepo to the controller's processing queue when the local cluster is updated.
// It ensures the timely installation or uninstallation of the system-upgrade-controller app in the local cluster
// when the "rancher.io/imported-cluster-version-management" annotation is changed.
func (h *handler) onCluster(_ string, obj *v3.Cluster) (*v3.Cluster, error) {
	if !features.MCM.Enabled() {
		return obj, nil
	}
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != "local" {
		return obj, nil
	}
	h.clusterRepo.EnqueueAfter(repoName, 2*time.Second)
	return obj, nil
}

// setPriorityClass attempts to retrieve the priority class for rancher pods and set it in the specified map
func (h *handler) setPriorityClass(values map[string]interface{}, chartName string) {
	priorityClassName, err := h.chartsConfig.GetGlobalValue(chart.PriorityClassKey)
	if err == nil {
		values[chart.PriorityClassKey] = priorityClassName
	} else if !chart.IsNotFoundError(err) {
		logrus.Warnf("[system-charts] failed to get %s for chart %q: %v", chart.PriorityClassKey, chartName, err)
	}
}

func (h *handler) setImagePullSecrets(values map[string]interface{}, useObjectReferences bool) {
	var pullSecretsAsStrings []string
	var pullSecretsAsObjectReferences []kcorev1.LocalObjectReference

	registry, _ := cluster.GetPrivateRegistry(nil)
	if registry != nil {
		pullSecretsAsObjectReferences = registry.PullSecretsAsObjectReferences()
		pullSecretsAsStrings = registry.PullSecretNamesAsSlice()
	}

	if useObjectReferences {
		values["imagePullSecrets"] = pullSecretsAsObjectReferences
	} else {
		values["imagePullSecrets"] = pullSecretsAsStrings
	}
}

// getChartValues attempts to retrieve chart values for the specified chart
func (h *handler) getChartValues(chartName string) map[string]interface{} {
	configMapValues, err := h.chartsConfig.GetChartValues(chartName)
	if err != nil && !chart.IsNotFoundError(err) {
		logrus.Warnf("[system-charts] failed to get chart values for chart %q: %v", chartName, err)
	}
	return configMapValues
}

func relatedFeatures(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if _, ok := obj.(*v3.Feature); ok {
		return []relatedresource.Key{{
			Name: repoName,
		}}, nil
	}
	return nil, nil
}

func relatedSettings(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if f, ok := obj.(*v3.Setting); ok {
		if _, ok := watchedSettings[f.Name]; ok {
			return []relatedresource.Key{{
				Name: repoName,
			}}, nil
		}
	}
	return nil, nil
}

func relatedConfigMaps(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if configMap, ok := obj.(*v1.ConfigMap); ok && configMap.Namespace == namespace.System && (configMap.Name == chart.CustomValueMapName || configMap.Name == settings.ConfigMapName.Get()) {
		return []relatedresource.Key{{
			Name: repoName,
		}}, nil
	}
	return nil, nil
}

func isInHarvesterLocal() bool {
	// When Rancher is embedded and running in the Harvester local cluster,
	// the multi-cluster-management and multi-cluster-management-agent features are disabled,
	// and the Harvester feature is enabled.
	if !features.MCMAgent.Enabled() && !features.MCM.Enabled() && features.Harvester.Enabled() {
		logrus.Debugf("[system-charts] Rancher is embedded in the Harvester local cluster")
		return true
	}
	return false
}

// removeCAPIWebhooks runs when the cattle-provisioning-capi-system namespace is deleted
// and ensures the old Rancher-created mutating/validating webhook configurations are removed
// after the provisioning CAPI chart is replaced by the turtles chart on upgrade.
func (h *handler) removeCAPIWebhooks(_ string, ns *kcorev1.Namespace) (*kcorev1.Namespace, error) {
	if ns == nil || ns.Name != "cattle-provisioning-capi-system" || ns.DeletionTimestamp == nil {
		return ns, nil
	}
	if err := h.mutatingWebhookConfigurations.Delete("capi-mutating-webhook-configuration", &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if err := h.validatingWebhookConfiguration.Delete("capi-validating-webhook-configuration", &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	return ns, nil
}
