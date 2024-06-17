package fleetcharts

import (
	"context"
	"os"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/data"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const priorityClassKey = "priorityClassName"

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

	watchedSettings = map[string]struct{}{
		settings.ServerURL.Name:             {},
		settings.CACerts.Name:               {},
		settings.SystemDefaultRegistry.Name: {},
		settings.FleetMinVersion.Name:       {},
		settings.FleetVersion.Name:          {},
		settings.AgentTLSMode.Name:          {},
	}
)

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager:      wContext.SystemChartsManager,
		chartsConfig: chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
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
	manager      chart.Manager
	chartsConfig chart.RancherConfigGetter
}

func (h *handler) onSetting(key string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil {
		return nil, nil
	}

	if _, isWatched := watchedSettings[setting.Name]; !isWatched {
		return setting, nil
	}

	h.Lock()
	if err := h.manager.Uninstall(fleetUninstallChart.ReleaseNamespace, fleetUninstallChart.ChartName); err != nil {
		h.Unlock()
		return nil, err
	}
	h.Unlock()

	fleetVersion := settings.FleetVersion.Get()
	// Keep Fleet min version precedence for backward compatibility.
	if fleetMinVersion := settings.FleetMinVersion.Get(); fleetMinVersion != "" {
		fleetVersion = fleetMinVersion
	}

	err := h.manager.Ensure(
		fleetCRDChart.ReleaseNamespace,
		fleetCRDChart.ChartName,
		fleetVersion,
		"",
		nil,
		true,
		"")
	if err != nil {
		return setting, err
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}

	fleetChartValues := map[string]interface{}{
		"agentTLSMode": settings.AgentTLSMode.Get(),
		"apiServerURL": settings.ServerURL.Get(),
		"apiServerCA":  settings.CACerts.Get(),
		"global":       systemGlobalRegistry,
		"bootstrap": map[string]interface{}{
			"enabled":        false,
			"agentNamespace": fleetconst.ReleaseLocalNamespace,
		},
		"gitops": map[string]interface{}{
			"enabled": features.Gitops.Enabled(),
		},
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

	// add priority class value
	if priorityClassName, err := h.chartsConfig.GetGlobalValue(chart.PriorityClassKey); err != nil {
		if !chart.IsNotFoundError(err) {
			logrus.Warnf("Failed to get rancher priorityClassName for '%s': %v", fleetChart.ChartName, err)
		}
	} else {
		fleetChartValues[priorityClassKey] = priorityClassName
		gitjobChartValues[priorityClassKey] = priorityClassName
	}

	if len(gitjobChartValues) > 0 {
		fleetChartValues["gitjob"] = gitjobChartValues
	}

	extraValues, err := h.chartsConfig.GetChartValues(fleetconst.ChartName)
	if err != nil && !chart.IsNotFoundError(err) {
		// Missing extra config is okay, return the error otherwise
		return nil, err
	}
	fleetChartValues = data.MergeMaps(fleetChartValues, extraValues)

	return setting,
		h.manager.Ensure(
			fleetChart.ReleaseNamespace,
			fleetChart.ChartName,
			fleetVersion,
			"",
			fleetChartValues,
			true,
			"")
}
