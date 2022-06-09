package mcmagent

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementagent"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	userContext, err := config.NewUserOnlyContext(wrangler)
	if err != nil {
		return err
	}
	return managementagent.Register(ctx, userContext)
}
