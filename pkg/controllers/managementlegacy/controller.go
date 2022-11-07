package managementlegacy

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/catalog"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/globaldns"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/multiclusterapp"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	catalog.Register(ctx, management)
	compose.Register(ctx, management, manager)
	globaldns.Register(ctx, management)
	multiclusterapp.Register(ctx, management, manager)
}
