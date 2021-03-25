package dashboard

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard/apiservice"
	"github.com/rancher/rancher/pkg/controllers/dashboard/fleetcharts"
	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	"github.com/rancher/rancher/pkg/controllers/dashboard/scaleavailable"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/needacert"
)

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	helm.Register(ctx, wrangler)
	kubernetesprovider.Register(ctx,
		wrangler.Mgmt.Cluster(),
		wrangler.K8s,
		wrangler.MultiClusterManager)
	apiservice.Register(ctx, wrangler)
	needacert.Register(ctx,
		wrangler.Core.Secret(),
		wrangler.Core.Service(),
		wrangler.Admission.MutatingWebhookConfiguration(),
		wrangler.Admission.ValidatingWebhookConfiguration(),
		wrangler.CRD.CustomResourceDefinition())
	scaleavailable.Register(ctx, wrangler)
	if features.Fleet.Enabled() {
		return fleetcharts.Register(ctx, wrangler)
	}
	return nil
}
