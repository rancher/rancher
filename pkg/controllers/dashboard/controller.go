package dashboard

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	helm.Register(ctx, wrangler)
	return nil
}
