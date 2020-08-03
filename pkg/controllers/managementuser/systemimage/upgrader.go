package systemimage

import (
	"github.com/rancher/rancher/pkg/types/config"
)

type SystemService interface {
	Init(cluster *config.UserContext)
	Upgrade(currentVersion string) (newVersion string, err error)
	Version() (string, error)
}
