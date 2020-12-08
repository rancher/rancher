package systemimage

import (
	"github.com/rancher/rancher/pkg/multiclustermanager/catalog/manager"
	"github.com/rancher/rancher/pkg/types/config"
)

type SystemService interface {
	Init(cluster *config.UserContext, catalogManager manager.CatalogManager)
	Upgrade(currentVersion string) (newVersion string, err error)
	Version() (string, error)
}
