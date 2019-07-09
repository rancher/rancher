package systemimage

import (
	"github.com/rancher/types/config"
)

type SystemService interface {
	Init(cluster *config.UserContext)
	Upgrade(currentVersion string) (newVersion string, err error)
	Version() (string, error)
}
