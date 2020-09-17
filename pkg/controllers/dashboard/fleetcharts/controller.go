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
	}
	fleetChart = chartDef{
		ReleaseNamespace: "fleet-system",
		ChartName:        "fleet",
	}
)

type chartDef struct {
	ReleaseNamespace string
	ChartName        string
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
}

func (h *handler) onSetting(key string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil {
		return nil, nil
	}

	if setting.Name != settings.ServerURL.Name &&
		setting.Name != settings.CACerts.Name {
		return setting, nil
	}

	err := h.manager.Ensure(fleetCRDChart.ReleaseNamespace, fleetCRDChart.ChartName, nil)
	if err != nil {
		return setting, err
	}

	return setting, h.manager.Ensure(fleetChart.ReleaseNamespace, fleetChart.ChartName,
		map[string]interface{}{
			"apiServerURL": settings.ServerURL.Get(),
			"apiServerCA":  settings.CACerts.Get(),
		})
}
