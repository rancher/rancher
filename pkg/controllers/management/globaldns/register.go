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

	p := newGlobalDNSProviderLauncher(ctx, management)
	if p != nil {
		management.Management.GlobalDNSProviders("").AddHandler(ctx, GlobaldnsProviderLauncher, p.sync)
	}

}
