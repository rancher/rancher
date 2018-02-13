package workload

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
)

func ConfigureStore(schemas *types.Schemas) {
	workloadSchema := schemas.Schema(&schema.Version, "workload")
	store := NewAggregateStore(
		schemas.Schema(&schema.Version, "deployment"),
		schemas.Schema(&schema.Version, "replicaSet"),
		schemas.Schema(&schema.Version, "replicationController"),
		schemas.Schema(&schema.Version, "daemonSet"),
		schemas.Schema(&schema.Version, "statefulSet"),
		schemas.Schema(&schema.Version, "job"),
		schemas.Schema(&schema.Version, "cronJob"))

	for _, s := range store.Schemas {
		s.Formatter = workloadFormatter
	}

	workloadSchema.Store = store
}

func workloadFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	workloadID := resource.ID
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	resource.Links["self"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	resource.Links["remove"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	resource.Links["update"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
}
