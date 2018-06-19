package app

import (
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/types/config"
)

func addListenConfig(management *config.ManagementContext, cfg Config) error {
	return tls.SetupListenConfig(management.Management.ListenConfigs(""), cfg.NoCACerts, cfg.ListenConfig)
}
