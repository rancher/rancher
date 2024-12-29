// Package systemcharts handles the reconciliation of systemcharts installed by rancher in the rancher-charts repo.
package systemcharts

import (
	"context"
	"fmt"
	"slices"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/features"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/data"
	deploymentControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	k8sappsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	repoName           = "rancher-charts"
	sucDeploymentName  = "system-upgrade-controller"
	legacyAppLabel     = "io.cattle.field/appId"
	legacyAppFinalizer = "systemcharts.cattle.io/legacy-k3s-based-upgrader-deprecation"
)

var (
	primaryImages = map[string]string{
		chart.WebhookChartName:                 "rancher/rancher-webhook",
		chart.ProvisioningCAPIChartName:        "rancher/mirrored-cluster-api-controller",
		chart.SystemUpgradeControllerChartName: "rancher/system-upgrade-controller",
	}
	watchedSettings = map[string]struct{}{
		settings.RancherWebhookVersion.Name:               {},
		settings.RancherProvisioningCAPIVersion.Name:      {},
		settings.SystemDefaultRegistry.Name:               {},
		settings.ShellImage.Name:                          {},
		settings.SystemUpgradeControllerChartVersion.Name: {},
	}
)

// Register is called to create a new handler and subscribe to change events.
func Register(ctx context.Context, wContext *wrangler.Context, registryOverride string) error {
	h := &handler{
		manager:          wContext.SystemChartsManager,
		namespaces:       wContext.Core.Namespace(),
		deployment:       wContext.Apps.Deployment(),
		deploymentCache:  wContext.Apps.Deployment().Cache(),
		clusterRepo:      wContext.Catalog.ClusterRepo(),
		chartsConfig:     chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
		registryOverride: registryOverride,
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-charts", h.onRepo)
	relatedresource.WatchClusterScoped(ctx, "bootstrap-charts", relatedFeatures, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Feature())

	relatedresource.WatchClusterScoped(ctx, "bootstrap-settings-charts", relatedSettings, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Setting())

	// ensure the system charts are installed with the correct values when there are changes to the rancher config map
	relatedresource.WatchClusterScoped(ctx, "bootstrap-configmap-charts", relatedConfigMaps, wContext.Catalog.ClusterRepo(), wContext.Core.ConfigMap())

	wContext.Apps.Deployment().OnChange(ctx, "legacy-k3sBasedUpgrader-deprecation", h.onDeployment)
	return nil
}

type handler struct {
	manager          chart.Manager
	namespaces       corecontrollers.NamespaceController
	deployment       deploymentControllers.DeploymentController
	deploymentCache  deploymentControllers.DeploymentCache
	clusterRepo      catalogcontrollers.ClusterRepoController
	chartsConfig     chart.RancherConfigGetter
	registryOverride string
}

func (h *handler) onRepo(key string, repo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	if repo == nil || repo.Name != repoName {
		return repo, nil
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}
	if h.registryOverride != "" {
		// if we have a specific image override, don't set the system default registry
		// don't need to check for type assert since we just created this above
		registryMap := systemGlobalRegistry["cattle"].(map[string]interface{})
		registryMap["systemDefaultRegistry"] = ""
		systemGlobalRegistry["cattle"] = registryMap
	}
	for _, chartDef := range h.getChartsToInstall() {
		if chartDef.Enabled != nil && !chartDef.Enabled() {
			continue
		}

		if chartDef.Uninstall {
			if err := h.manager.Uninstall(chartDef.ReleaseNamespace, chartDef.ChartName); err != nil {
				return repo, err
			}
			if chartDef.RemoveNamespace {
				if err := h.namespaces.Delete(chartDef.ReleaseNamespace, nil); err != nil && !errors.IsNotFound(err) {
					return repo, err
				}
			}
			continue
		}

		values := map[string]interface{}{
			"global": systemGlobalRegistry,
		}
		var installImageOverride string
		if h.registryOverride != "" {
			imageSettings, ok := values["image"].(map[string]interface{})
			if !ok {
				imageSettings = map[string]interface{}{}
			}
			if image, ok := primaryImages[chartDef.ChartName]; ok {
				imageSettings["repository"] = h.registryOverride + "/" + image
			}
			values["image"] = imageSettings
			installImageOverride = h.registryOverride + "/" + settings.ShellImage.Get()
		}
		if chartDef.Values != nil {
			for k, v := range chartDef.Values() {
				values[k] = v
			}
		}
		// webhook needs to be able to adopt the MutatingWebhookConfiguration which originally wasn't a part of the
		// chart definition, but is now part of the chart definition
		minVersion := chartDef.MinVersionSetting.Get()
		exactVersion := chartDef.ExactVersionSetting.Get()
		takeOwnership := chartDef.ChartName == chart.WebhookChartName || chartDef.ChartName == chart.ProvisioningCAPIChartName
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, minVersion, exactVersion, values, takeOwnership, installImageOverride); err != nil {
			return repo, err
		}
	}

	return repo, nil
}

func (h *handler) getChartsToInstall() []*chart.Definition {
	return []*chart.Definition{
		{
			ReleaseNamespace:    namespace.System,
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
			ChartName:        "rancher-operator",
			Uninstall:        true,
			RemoveNamespace:  true,
		},
		{
			ReleaseNamespace:    namespace.ProvisioningCAPINamespace,
			ChartName:           chart.ProvisioningCAPIChartName,
			ExactVersionSetting: settings.RancherProvisioningCAPIVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{}
				// add priority class value
				h.setPriorityClass(values, chart.ProvisioningCAPIChartName)
				// get custom values for the rancher-provisioning-capi
				configMapValues := h.getChartValues(chart.ProvisioningCAPIChartName)
				return data.MergeMaps(values, configMapValues)
			},
			Enabled:         func() bool { return true },
			Uninstall:       !features.EmbeddedClusterAPI.Enabled(),
			RemoveNamespace: !features.EmbeddedClusterAPI.Enabled(),
		},
		{
			ReleaseNamespace:    namespace.System,
			ChartName:           chart.SystemUpgradeControllerChartName,
			ExactVersionSetting: settings.SystemUpgradeControllerChartVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{}
				// add priority class value
				h.setPriorityClass(values, chart.SystemUpgradeControllerChartName)
				// get custom values for system-upgrade-controller
				configMapValues := h.getChartValues(chart.SystemUpgradeControllerChartName)
				return data.MergeMaps(values, configMapValues)
			},
			Enabled: func() bool {
				toEnable := false
				suc, err := h.deploymentCache.Get(namespace.System, sucDeploymentName)

				// the absence of the deployment or the absence of the legacy label on the existing deployment indicate
				// that the old rancher-k3s/rke2-upgrader Project App has been removed
				if err != nil {
					if errors.IsNotFound(err) {
						toEnable = true
					} else {
						logrus.Warnf("[systemcharts] failed to get the deployment %s/%s: %v", namespace.System, sucDeploymentName, err)
					}
				}
				if suc != nil {
					if _, ok := suc.Labels[legacyAppLabel]; !ok {
						toEnable = true
					}
				}
				logrus.Infof("[systemcharts] feature ManagedSystemUpgradeController = %t, toEnable = %t",
					features.ManagedSystemUpgradeController.Enabled(), toEnable)

				return features.ManagedSystemUpgradeController.Enabled() && toEnable
			},
		},
	}
}

// onDeployment enqueues the rancher-charts ClusterRepo to the controller's processing queue
// when a specific event occurs on the target deployment. It is currently used to manage
// the migration from the legacy k3s-based-upgrader app to the system-upgrade-controller app.
func (h *handler) onDeployment(_ string, d *k8sappsv1.Deployment) (*k8sappsv1.Deployment, error) {
	if d == nil || d.Namespace != namespace.System || d.Name != sucDeploymentName {
		return d, nil
	}
	if appName, ok := d.Labels[legacyAppLabel]; !ok || (appName != k3sbasedupgrade.K3sAppName && appName != k3sbasedupgrade.Rke2AppName) {
		return d, nil
	}
	index := slices.Index(d.Finalizers, legacyAppFinalizer)
	logrus.Infof("[systemcharts] found deployment %s/%s with label %s=%s, index of target finalzier = %d",
		d.Namespace, d.Name, legacyAppLabel, d.Labels[legacyAppLabel], index)
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
		logrus.Infof("[systemcharts] enqueue %s", repoName)
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

// setPriorityClass attempts to retrieve the priority class for rancher pods and set it in the specified map
func (h *handler) setPriorityClass(values map[string]interface{}, chartName string) {
	priorityClassName, err := h.chartsConfig.GetGlobalValue(chart.PriorityClassKey)
	if err == nil {
		values[chart.PriorityClassKey] = priorityClassName
	} else if !chart.IsNotFoundError(err) {
		logrus.Warnf("[systemcharts] Failed to get rancher %s for %s: %s", chart.PriorityClassKey, chartName, err.Error())
	}
}

// getChartValues attempts to retrieve chart values for the specified chart
func (h *handler) getChartValues(chartName string) map[string]interface{} {
	configMapValues, err := h.chartsConfig.GetChartValues(chartName)
	if err != nil && !chart.IsNotFoundError(err) {
		logrus.Warnf("[systemcharts] Failed to get chart values for %s: %s", chartName, err.Error())
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
