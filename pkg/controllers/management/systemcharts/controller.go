package systemcharts

import (
	"context"
	"sync"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

var (
	toInstall = []chartDef{
		{
			ReleaseNamespace:  namespaces.System,
			ChartName:         "rancher-webhook",
			MinVersionSetting: settings.RancherWebhookMinVersion,
		},
		{
			ReleaseNamespace:  "rancher-operator-system",
			ChartName:         "rancher-operator-crd",
			MinVersionSetting: settings.RancherOperatorMinVersion,
		},
		{
			ReleaseNamespace:  "rancher-operator-system",
			ChartName:         "rancher-operator",
			MinVersionSetting: settings.RancherOperatorMinVersion,
		},
	}
	repoName = "rancher-charts"
)

type chartDef struct {
	ReleaseNamespace  string
	ChartName         string
	MinVersionSetting settings.Setting
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-charts", h.onRepo)
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
		if err := h.manager.Ensure(chartDef.ReleaseNamespace, chartDef.ChartName, chartDef.MinVersionSetting.Get(), map[string]interface{}{
			"global": systemGlobalRegistry,
		}); err != nil {
			return repo, err
		}
	}

	return repo, nil
}
