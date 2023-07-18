// Package systemcharts handles the reconciliation of systemcharts installed by rancher in the rancher-charts repo.
package systemcharts

import (
	"context"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/features"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/data"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	repoName         = "rancher-charts"
	webhookImage     = "rancher/rancher-webhook"
	priorityClassKey = "priorityClassName"
)

var (
	watchedSettings = map[string]struct{}{
		settings.RancherWebhookMinVersion.Name: {},
		settings.RancherWebhookVersion.Name:    {},
		settings.SystemDefaultRegistry.Name:    {},
		settings.ShellImage.Name:               {},
	}
)

// Register is called to create a new handler and subscribe to change events.
func Register(ctx context.Context, wContext *wrangler.Context, registryOverride string) error {
	h := &handler{
		manager:          wContext.SystemChartsManager,
		namespaces:       wContext.Core.Namespace(),
		chartsConfig:     chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
		registryOverride: registryOverride,
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-charts", h.onRepo)
	relatedresource.WatchClusterScoped(ctx, "bootstrap-charts", relatedFeatures, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Feature())

	relatedresource.WatchClusterScoped(ctx, "bootstrap-settings-charts", relatedSettings, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Setting())

	// ensure the system charts are installed with the correct values when there are changes to the rancher config map
	relatedresource.WatchClusterScoped(ctx, "bootstrap-configmap-charts", relatedConfigMaps, wContext.Catalog.ClusterRepo(), wContext.Core.ConfigMap())
	return nil
}

type handler struct {
	manager          chart.Manager
	namespaces       corecontrollers.NamespaceController
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
			imageSettings["repository"] = h.registryOverride + "/" + webhookImage
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
		if chartDef.ChartName == chart.WebhookChartName && minVersion != "" {
			exactVersion = ""
		}
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, minVersion, exactVersion, values, chartDef.ChartName == chart.WebhookChartName, installImageOverride); err != nil {
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
			MinVersionSetting:   settings.RancherWebhookMinVersion,
			ExactVersionSetting: settings.RancherWebhookVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
				}
				// add priority class value
				if priorityClassName, err := h.chartsConfig.GetGlobalValue(chart.PriorityClassKey); err != nil {
					if !chart.IsNotFoundError(err) {
						logrus.Warnf("Failed to get rancher %s for 'rancher-webhook': %s", chart.PriorityClassKey, err.Error())
					}
				} else {
					values[priorityClassKey] = priorityClassName
				}

				// get custom values for the rancher-webhook
				configMapValues, err := h.chartsConfig.GetChartValues(chart.WebhookChartName)
				if err != nil && !chart.IsNotFoundError(err) {
					logrus.Warnf("Failed to get rancher rancherWebhookValues %s", err.Error())
				}

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
	}
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
