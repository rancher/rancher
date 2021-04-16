package provisioningv2

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/bootstrap"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/dynamicschema"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/machineprovision"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/planstatus"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/ranchercluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/rkecluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/unmanaged"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/provisioningv2/capi"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, clients *wrangler.Context) error {
	cluster.Register(ctx, clients)

	if features.RKE2.Enabled() {
		if features.MCM.Enabled() {
			dynamicschema.Register(ctx, clients)
		}
		rkecluster.Register(ctx, clients)
		ranchercluster.Register(ctx, clients)
		bootstrap.Register(ctx, clients)
		machineprovision.Register(ctx, clients)
		planner.Register(ctx, clients)
		planstatus.Register(ctx, clients)
		unmanaged.Register(ctx, clients)
	}

	if features.EmbeddedClusterAPI.Enabled() {
		capiStart, err := capi.Register(ctx, clients)
		if err != nil {
			return err
		}
		clients.OnLeader(func(ctx context.Context) error {
			if err := capiStart(ctx); err != nil {
				logrus.Fatal(err)
			}
			logrus.Info("Cluster API is started")
			return nil
		})
	}

	return nil
}
