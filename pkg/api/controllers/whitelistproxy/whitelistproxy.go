package whitelistproxy

import (
	"github.com/rancher/rancher/server/whitelist"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

func Register(management *config.ScaledContext) {
	management.Management.NodeDrivers("").AddHandler("whitelist-proxy", sync)
}

func sync(key string, nodeDriver *v3.NodeDriver) error {
	if key == "" || nodeDriver == nil {
		return nil
	}
	if nodeDriver.DeletionTimestamp != nil {
		for _, d := range nodeDriver.Spec.WhitelistDomains {
			whitelist.Proxy.Rm(d)
		}
		return nil
	}

	for _, d := range nodeDriver.Spec.WhitelistDomains {
		whitelist.Proxy.Add(d)
	}
	return nil
}
