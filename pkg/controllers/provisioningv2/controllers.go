package provisioningv2

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/fleetcluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/fleetworkspace"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/machineconfigcleanup"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/managedchart"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/provisioningcluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/provisioninglog"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/secret"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, clients *wrangler.Context, kubeconfigManager *kubeconfig.Manager) {
	cluster.Register(ctx, clients, kubeconfigManager)
	if features.MCM.Enabled() {
		secret.Register(ctx, clients)
	}
	provisioningcluster.Register(ctx, clients)
	provisioninglog.Register(ctx, clients)
	machineconfigcleanup.Register(ctx, clients)

	if features.Fleet.Enabled() {
		managedchart.Register(ctx, clients)
		fleetcluster.Register(ctx, clients)
		fleetworkspace.Register(ctx, clients)
	}
}
