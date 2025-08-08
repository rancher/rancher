package provisioningv2

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/fleetcluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/fleetworkspace"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/harvestercleanup"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/machineconfigcleanup"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/managedchart"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/provisioningcluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/provisioninglog"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/secret"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, clients *wrangler.Context, kubeconfigManager *kubeconfig.Manager) {
	if features.MCM.Enabled() {
		secret.Register(ctx, clients)
	}
	provisioninglog.Register(ctx, clients)
	machineconfigcleanup.Register(ctx, clients)

	if features.Fleet.Enabled() {
		managedchart.Register(ctx, clients)
		fleetcluster.Register(ctx, clients)
		fleetworkspace.Register(ctx, clients)
	}

	go func() {
		logrus.Debug("[provisioningv2] Waiting for CAPI CRDs to be available")
		if !clients.WaitForCAPICRDs(ctx) {
			return
		}
		logrus.Debug("[provisioningv2] CAPI CRDs are available, proceeding with initialization")

		if err := clients.InitializeCAPIFactory(ctx); err != nil {
			logrus.Errorf("[provisioningv2] Failed to initialize CAPI factory: %v", err)
			return
		}

		cluster.Register(ctx, clients, kubeconfigManager)
		provisioningcluster.Register(ctx, clients)

		if features.Harvester.Enabled() {
			harvestercleanup.Register(ctx, clients)
		}
		logrus.Debug("[provisioningv2] All CAPI dependent controllers registered successfully")
	}()
}
