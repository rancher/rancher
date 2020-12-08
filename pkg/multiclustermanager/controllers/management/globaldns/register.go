package globaldns

import (
	"context"

	"github.com/rancher/rancher/pkg/multiclustermanager/catalog/manager"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext, catalogManager manager.CatalogManager) {
	n := newGlobalDNSController(ctx, management)
	if n != nil {
		management.Management.GlobalDnses("").AddHandler(ctx, GlobaldnsController, n.sync)
	}

	cp := newGlobalDNSProviderCatalogLauncher(ctx, management, catalogManager)
	if cp != nil {
		management.Management.GlobalDnsProviders("").AddHandler(ctx, GlobaldnsProviderCatalogLauncher, cp.sync)
	}

	sp := newProviderSecretSyncer(ctx, management)
	if sp != nil {
		management.Core.Secrets("").AddHandler(ctx, GlobaldnsProviderSecretSyncer, sp.sync)
	}

}
