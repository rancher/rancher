package globaldns

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	n := newGlobalDNSController(ctx, management)
	if n != nil {
		management.Management.GlobalDnses("").AddHandler(ctx, GlobaldnsController, n.sync)
	}

	cp := newGlobalDNSProviderCatalogLauncher(ctx, management)
	if cp != nil {
		management.Management.GlobalDNSProviders("").AddHandler(ctx, GlobaldnsProviderCatalogLauncher, cp.sync)
	}

	sp := newProviderSecretSyncer(ctx, management)
	if sp != nil {
		management.Core.Secrets("").AddHandler(ctx, GlobaldnsProviderSecretSyncer, sp.sync)
	}

}
