package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
)

var (
	Version = types.APIVersion{
		Version: "v1",
		Group:   "provisioning.cattle.io",
		Path:    "/v1",
	}

	Schemas = ProvisioningSchemas(&Version).
		Init(clusterTypes)
)

func clusterTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.AddMapperForType(&Version, v1.RKEConfig{},
		m.Drop{Field: "chartValues"},
		&m.ChangeType{Field: "machineGlobalConfig", Type: "MachineGlobalConfig"}).
		MustImport(&Version, MachineGlobalConfig{}).
		AddMapperForType(&Version, v1.RKEMachinePool{},
			&m.ChangeType{Field: "drainBeforeDeleteTimeout", Type: "string"},
			&m.ChangeType{Field: "nodeStartupTimeout", Type: "string"},
			&m.ChangeType{Field: "unhealthyNodeTimeout", Type: "string"}).
		MustImportAndCustomize(&Version, v1.RKEMachinePool{}, func(schema *types.Schema) {}).
		MustImportAndCustomize(&Version, v1.Cluster{}, func(schema *types.Schema) {
			schema.ID = "provisioning.cattle.io.cluster"
		})
}
