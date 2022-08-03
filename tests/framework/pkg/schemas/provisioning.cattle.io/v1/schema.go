package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		MustImportAndCustomize(&Version, v1.RKEConfig{}, func(schema *types.Schema) {}).
		MustImportAndCustomize(&Version, metav1.Duration{}, func(schema *types.Schema) {}, Duration{}).
		MustImportAndCustomize(&Version, v1.Cluster{}, func(schema *types.Schema) {
			schema.ID = "provisioning.cattle.io.cluster"
		})
}
