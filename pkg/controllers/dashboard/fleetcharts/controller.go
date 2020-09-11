package fleetcharts

import (
	"context"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

var (
	fleetCRDChart = chartDef{
		ReleaseNamespace: "fleet-system",
		ChartName:        "fleet-crd",
		Version:          "~0-a",
	}
	fleetChart = chartDef{
		ReleaseNamespace: "fleet-system",
		ChartName:        "fleet",
		Version:          "~0-a",
	}
	repoName = "rancher-charts"
)

type chartDef struct {
	ReleaseNamespace string
	ChartName        string
	Version          string
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
	}

	wContext.Mgmt.Setting().OnChange(ctx, "fleet-install", h.onSetting)
	return nil
}

type handler struct {
	sync.Mutex
	manager *system.Manager
	once    sync.Once
}

func (h *handler) onSetting(key string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil {
		return nil, nil
	}

	if setting.Name != settings.ServerURL.Name &&
		setting.Name != settings.CACerts.Name {
		return setting, nil
	}

	h.Lock()
	defer h.Unlock()

	err := h.manager.Ensure(fleetCRDChart.ReleaseNamespace, fleetCRDChart.ChartName, fleetCRDChart.Version, nil)
	if err != nil {
		return setting, err
	}

	return setting, h.manager.Ensure(fleetChart.ReleaseNamespace, fleetChart.ChartName, fleetChart.Version,
		map[string]interface{}{
			"apiServerURL": settings.ServerURL.Get(),
			"apiServerCA":  settings.CACerts.Get(),
		})
}
