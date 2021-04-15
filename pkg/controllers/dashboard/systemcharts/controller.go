package systemcharts

import (
	"context"
	"sync"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/features"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	toInstall = []chartDef{
		{
			ReleaseNamespace:  namespaces.System,
			ChartName:         "rancher-webhook",
			MinVersionSetting: settings.RancherWebhookMinVersion,
			Values: func() map[string]interface{} {
				return map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
				}
			},
			Enabled: func() bool {
				return features.MCM.Enabled() || features.EmbeddedClusterAPI.Enabled()
			},
		},
		{
			ReleaseNamespace: "rancher-operator-system",
			ChartName:        "rancher-operator",
			Uninstall:        true,
		},
	}
	repoName = "rancher-charts"
)

type chartDef struct {
	ReleaseNamespace  string
	ChartName         string
	MinVersionSetting settings.Setting
	Values            func() map[string]interface{}
	Enabled           func() bool
	Uninstall         bool
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
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
	manager *system.Manager
	once    sync.Once
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
	for _, chartDef := range toInstall {
		if chartDef.Enabled != nil && !chartDef.Enabled() {
			continue
		}

		if chartDef.Uninstall {
			if err := h.manager.Uninstall(chartDef.ReleaseNamespace, chartDef.ChartName); err != nil {
				return repo, err
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
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, chartDef.MinVersionSetting.Get(), values); err != nil {
			return repo, err
		}
	}

	return repo, nil
}
