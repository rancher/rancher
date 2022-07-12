package schema

import (
	"github.com/rancher/norman/types"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
)

var (
	Version = types.APIVersion{
		Version: "v1",
		Group:   "rke.cattle.io",
		Path:    "/v1",
	}

	Schemas = RKESchemas(&Version).
		Init(rkeTypes)
)

func rkeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v1.RKEControlPlane{})
}
