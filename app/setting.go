package app

import (
	"github.com/rancher/types/config"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/pborman/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
)

func addSetting(management *config.ManagementContext) error {
	settingClient := management.Management.Settings("")
	installUUID := &v3.Setting{}
	installUUID.Default = uuid.NewUUID().String()
	installUUID.Name = "install-uuid"
	if _, err := settingClient.Create(installUUID); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
