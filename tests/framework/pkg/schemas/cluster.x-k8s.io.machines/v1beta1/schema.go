package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	Version = types.APIVersion{
		Version: capi.GroupVersion.Version,
		Group:   capi.GroupVersion.Group,
		Path:    "/v1",
	}

	Schemas = ClusterMachineSchemas(&Version).
		Init(clusterMachineTypes)
)

func clusterMachineTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.AddMapperForType(&Version, capi.MachineSpec{},
		&m.ChangeType{Field: "nodeDeletionTimeout", Type: "string"},
		&m.ChangeType{Field: "nodeDrainTimeout", Type: "string"}).
		MustImportAndCustomize(&Version, capi.Machine{}, func(schema *types.Schema) {
			schema.ID = "cluster.x-k8s.io.machine"
		})
}
