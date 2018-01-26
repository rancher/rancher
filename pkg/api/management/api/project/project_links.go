package project

import (
	"github.com/rancher/norman/types"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func ClusterLinks(apiContext *types.APIContext, resource *types.RawResource) {
	if resource.Type == client.ClusterType {
		for _, schema := range clusterSchema.Schemas.Schemas() {
			if !schema.CanList(apiContext) {
				continue
			}
			resource.Links[schema.PluralName] = apiContext.URLBuilder.Link(schema.PluralName, resource)
		}

		resource.Links["namespaces"] = apiContext.URLBuilder.Link("namespaces", resource)
		resource.Links["schemas"] = apiContext.URLBuilder.Link("schemas", resource)

		for _, schema := range projectSchema.Schemas.Schemas() {
			if !schema.CanList(apiContext) {
				continue
			}
			if _, ok := schema.ResourceFields["projectId"]; ok {
				continue
			}
			resource.Links[schema.PluralName] = apiContext.URLBuilder.Link(schema.PluralName, resource)
		}
	} else if resource.Type == client.ProjectType {
		for _, schema := range projectSchema.Schemas.Schemas() {
			if !schema.CanList(apiContext) {
				continue
			}
			if _, ok := schema.ResourceFields["projectId"]; ok {
				resource.Links[schema.PluralName] = apiContext.URLBuilder.Link(schema.PluralName, resource)
			}
		}

		resource.Links["schemas"] = apiContext.URLBuilder.Link("schemas", resource)
	}
}
