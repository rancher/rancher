package management

import (
	"github.com/pborman/uuid"
	"github.com/rancher/rancher/pkg/settings"
)

func addSetting() error {
	return settings.InstallUUID.SetIfUnset(uuid.NewRandom().String())
}
