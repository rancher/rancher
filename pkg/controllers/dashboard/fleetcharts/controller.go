package fleetcharts

import (
	"context"
	"os"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	fleetCRDChart = chart.Definition{
		ReleaseNamespace: fleetconst.ReleaseNamespace,
		ChartName:        fleetconst.CRDChartName,
	}
	fleetChart = chart.Definition{
		ReleaseNamespace: fleetconst.ReleaseNamespace,
		ChartName:        fleetconst.ChartName,
	}
	fleetUninstallChart = chart.Definition{
		ReleaseNamespace: fleetconst.ReleaseLegacyNamespace,
		ChartName:        fleetconst.ChartName,
	}
)

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
	}

	wContext.Mgmt.Setting().OnChange(ctx, "fleet-install", h.onSetting)
	// watch cluster repo `rancher-charts` and enqueue the setting to make sure the latest fleet is installed after catalog refresh
	relatedresource.WatchClusterScoped(ctx, "bootstrap-fleet-charts", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if name == "rancher-charts" {
			return []relatedresource.Key{{
				Name: settings.ServerURL.Name,
			}}, nil
		}
		return nil, nil
	}, wContext.Mgmt.Setting(), wContext.Catalog.ClusterRepo())
	return nil
}

type handler struct {
	sync.Mutex
	manager *system.Manager
}

func (h *handler) onSetting(key string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil {
		return nil, nil
	}

	if setting.Name != settings.ServerURL.Name &&
		setting.Name != settings.CACerts.Name &&
		setting.Name != settings.SystemDefaultRegistry.Name {
		return setting, nil
	}

	h.Lock()
	if err := h.manager.Uninstall(fleetUninstallChart.ReleaseNamespace, fleetUninstallChart.ChartName); err != nil {
		h.Unlock()
		return nil, err
	}
	h.Unlock()

	err := h.manager.Ensure(fleetCRDChart.ReleaseNamespace, fleetCRDChart.ChartName, settings.FleetMinVersion.Get(), nil, true)
	if err != nil {
		return setting, err
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}

	fleetChartValues := map[string]interface{}{
		"apiServerURL": settings.ServerURL.Get(),
		"apiServerCA":  settings.CACerts.Get(),
		"global":       systemGlobalRegistry,
		"bootstrap": map[string]interface{}{
			"agentNamespace": fleetconst.ReleaseLocalNamespace,
		},
	}

	fleetChartValues["gitops"] = map[string]interface{}{
		"enabled": features.Gitops.Enabled(),
	}

	gitjobChartValues := make(map[string]interface{})

	if envVal, ok := os.LookupEnv("HTTP_PROXY"); ok {
		fleetChartValues["proxy"] = envVal
		gitjobChartValues["proxy"] = envVal
	}
	if envVal, ok := os.LookupEnv("NO_PROXY"); ok {
		fleetChartValues["noProxy"] = envVal
		gitjobChartValues["noProxy"] = envVal
	}

	if len(gitjobChartValues) > 0 {
		fleetChartValues["gitjob"] = gitjobChartValues
	}

	return setting, h.manager.Ensure(fleetChart.ReleaseNamespace, fleetChart.ChartName, settings.FleetMinVersion.Get(), fleetChartValues, true)
}
