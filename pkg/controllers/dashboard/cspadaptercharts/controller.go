// Package cspadaptercharts creates the controllers necessary for automatically upgrading the csp adapter chart
package cspadaptercharts

import (
	"context"
	"errors"
	"fmt"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/managedcharts/cspadapter"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

// Register registers a setting controller which watches the CSPAdapterMinVersion setting for changes so an installed CSP
// Adapter can be upgraded
func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager:     wContext.SystemChartsManager,
		adapterUtil: cspadapter.NewChartUtil(wContext.RESTClientGetter),
	}
	wContext.Mgmt.Setting().OnChange(ctx, "csp-adapter-upgrade", h.onSetting)

	return nil
}

type handler struct {
	sync.Mutex
	manager     *system.Manager
	adapterUtil *cspadapter.ChartUtil
}

func (h *handler) onSetting(key string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || setting.Name != settings.CSPAdapterMinVersion.Name {
		// we only need to check for changes to the CSPAdapterMinVersion setting, ignore any others
		return setting, nil
	}
	adapterRelease, err := h.adapterUtil.GetRelease(cspadapter.MLOChartNamespace, cspadapter.MLOChartName)
	if err != nil {
		if errors.Is(err, cspadapter.ErrNotFound) {
			// If the adapter isn't installed, don't attempt an upgrade
			return setting, nil
		}
		// if we can't determine the status of the adapter, stop here and attempt to re-evaluate later
		return setting, fmt.Errorf("unable to validate if the csp adater was installed: %w", err)
	}
	err = h.manager.Ensure(cspadapter.MLOChartNamespace, adapterRelease.Chart.Name(), settings.CSPAdapterMinVersion.Get(), "", nil, true, "")
	return setting, err
}
