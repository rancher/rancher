package globaldns

import (
	"context"

	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	n := newGlobalDNSController(ctx, management)
	if n != nil {
		management.Management.GlobalDNSs("").AddHandler(ctx, GlobaldnsController, n.sync)
	}

	cp := newGlobalDNSProviderCatalogLauncher(ctx, management)
	if cp != nil {
		management.Management.GlobalDNSProviders("").AddHandler(ctx, GlobaldnsProviderCatalogLauncher, cp.sync)
	}

}
