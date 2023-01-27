package systemcharts

import (
	"context"
	"strconv"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/features"
	namespace "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	repoName = "rancher-charts"
)

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager:      wContext.SystemChartsManager,
		namespaces:   wContext.Core.Namespace(),
		chartsConfig: chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-charts", h.onRepo)
	relatedresource.WatchClusterScoped(ctx, "bootstrap-charts", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		return []relatedresource.Key{{
			Name: repoName,
		}}, nil
	}, wContext.Catalog.ClusterRepo(), wContext.Mgmt.Feature())
	return nil
}

type handler struct {
	manager      chart.Manager
	namespaces   corecontrollers.NamespaceController
	chartsConfig chart.RancherConfigGetter
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
		if chartDef.Values != nil {
			for k, v := range chartDef.Values() {
				values[k] = v
			}
		}
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, chartDef.MinVersionSetting.Get(), values, false); err != nil {
			return repo, err
		}
	}

	return repo, nil
}

func (h *handler) getChartsToInstall() []*chart.Definition {
	return []*chart.Definition{
		{
			ReleaseNamespace:  namespace.System,
			ChartName:         "rancher-webhook",
			MinVersionSetting: settings.RancherWebhookMinVersion,
			Values: func() map[string]interface{} {
				values := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]any{
						"cattle": map[string]any{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
							"psp":                   map[string]any{},
						},
					},
				}
				// add priority class value.
				if priorityClassName, err := h.chartsConfig.GetPriorityClassName(); err != nil {
					if !apierror.IsNotFound(err) {
						logrus.Warnf("Failed to get rancher priorityClassName for 'rancher-webhook': %v", err)
					}
				} else {
					values[chart.PriorityClassKey] = priorityClassName
				}

				if pspEnabled, err := h.chartsConfig.GetPSPEnablement(); err != nil {
					if !apierror.IsNotFound(err) {
						logrus.Warnf("Failed to get pspEnablement value for 'rancher-webhook': %v", err)
					}
				} else {
					enabled, err := strconv.ParseBool(pspEnabled)
					if err != nil {
						logrus.Warnf("Failed to parse pspEnablement value as a bool for 'rancher-webhook'")
					} else {
						values["global"].(map[string]any)["cattle"].(map[string]any)["psp"].(map[string]any)["enabled"] = enabled
					}
				}

				return values
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
