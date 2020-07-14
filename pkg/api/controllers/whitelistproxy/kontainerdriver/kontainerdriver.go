package kontainerdriver

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/server/whitelist"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, management *config.ScaledContext) {
	management.Management.KontainerDrivers("").AddHandler(ctx, "whitelist-proxy", sync)
}

func sync(key string, kontainerDriver *v3.KontainerDriver) (runtime.Object, error) {
	if key == "" || kontainerDriver == nil {
		return nil, nil
	}
	if kontainerDriver.DeletionTimestamp != nil {
		for _, d := range kontainerDriver.Spec.WhitelistDomains {
			whitelist.Proxy.Rm(d)
		}
		return nil, nil
	}

	for _, d := range kontainerDriver.Spec.WhitelistDomains {
		whitelist.Proxy.Add(d)
	}
	return nil, nil
}
