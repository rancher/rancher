package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type CatalogInfo struct {
	Catalog        *v3.Catalog
	ProjectCatalog *v3.ProjectCatalog
	ClusterCatalog *v3.ClusterCatalog
}
