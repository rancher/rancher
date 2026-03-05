package proxysettings

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

type handler struct {
	clients *wrangler.Context
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clients: clients,
	}
	clients.Mgmt.Setting().OnChange(ctx, "handle-builtin-proxy-endpoints", h.manageDefaultProxyEndpoints)
}

func (h *handler) manageDefaultProxyEndpoints(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || setting.Name != settings.DisableDefaultProxyEndpoint.Name {
		return setting, nil
	}
	err := management.AddProxyEndpointData(setting.Value, h.clients)
	return setting, err
}
