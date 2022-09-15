package schema

import (
	"github.com/rancher/norman/types"
	provisioningSchema "github.com/rancher/rancher/tests/framework/pkg/schemas/provisioning.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return schemas.
		MustImportAndCustomize(&Version, metav1.Duration{}, func(schema *types.Schema) {}, provisioningSchema.Duration{}).
		MustImportAndCustomize(&Version, capi.Machine{}, func(schema *types.Schema) {
			schema.ID = "cluster.x-k8s.io.machine"
		})
}
