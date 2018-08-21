package app

import (
	apptypes "github.com/rancher/rancher/app/types"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/types/config"
)

func addListenConfig(management *config.ManagementContext, cfg apptypes.Config) error {
	return tls.SetupListenConfig(management.Management.ListenConfigs(""), cfg.NoCACerts, cfg.ListenConfig)
}
