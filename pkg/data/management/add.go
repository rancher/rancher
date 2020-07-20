package management

import (
	"github.com/rancher/rancher/pkg/auth/data"
	"github.com/rancher/rancher/pkg/types/config"
)

func Add(management *config.ManagementContext, addLocal, removeLocal, embedded bool) error {
	adminName, err := addRoles(management)
	if err != nil {
		return err
	}

	if addLocal {
		if err := addLocalCluster(embedded, adminName, management); err != nil {
			return err
		}
	} else if removeLocal {
		if err := removeLocalCluster(management); err != nil {
			return err
		}
	}

	if err := data.AuthConfigs(management); err != nil {
		return err
	}

	if err := syncCatalogs(management); err != nil {
		return err
	}

	if err := addSetting(); err != nil {
		return err
	}

	if err := addDefaultPodSecurityPolicyTemplates(management); err != nil {
		return err
	}

	if err := addKontainerDrivers(management); err != nil {
		return err
	}

	if err := addCattleGlobalNamespaces(management); err != nil {
		return err
	}

	return addMachineDrivers(management)
}
