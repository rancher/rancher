package catalog

import (
	"context"
	"net/http"

	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	"github.com/rancher/apiserver/pkg/types"
	types2 "github.com/rancher/rancher/pkg/api/steve/catalog/types"
	"github.com/rancher/rancher/pkg/apis/catalog.cattle.io"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	schemas3 "github.com/rancher/wrangler/pkg/schemas"
)

func Register(ctx context.Context, server *steve.Server,
	helmop *helmop.Operations,
	contentManager *content.Manager) error {
	ops := newOperation(helmop)
	server.ClusterCache.OnAdd(ctx, ops.OnAdd)
	server.ClusterCache.OnChange(ctx, ops.OnChange)

	index := &contentDownload{
		contentManager: contentManager,
	}

	addSchemas(server, ops, index)
	return nil
}

func addSchemas(server *steve.Server, ops *operation, index http.Handler) {
	server.BaseSchemas.MustImportAndCustomize(types2.ChartUninstallAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(types2.ChartUpgradeAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(types2.ChartUpgrade{}, nil)
	server.BaseSchemas.MustImportAndCustomize(types2.ChartInstallAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(types2.ChartInstall{}, nil)
	server.BaseSchemas.MustImportAndCustomize(types2.ChartActionOutput{}, nil)

	operationTemplate := schema2.Template{
		Group: catalog.GroupName,
		Kind:  "Operation",
		Customize: func(apiSchema *types.APISchema) {
			apiSchema.LinkHandlers = map[string]http.Handler{
				"logs": ops,
			}
			apiSchema.Formatter = func(request *types.APIRequest, resource *types.RawResource) {
				if !resource.APIObject.Data().Bool("status", "podCreated") {
					delete(resource.Links, "logs")
				}
			}
		},
	}
	appTemplate := schema2.Template{
		Group: catalog.GroupName,
		Kind:  "App",
		Customize: func(apiSchema *types.APISchema) {
			apiSchema.ActionHandlers = map[string]http.Handler{
				"uninstall": ops,
			}
			apiSchema.ResourceActions = map[string]schemas3.Action{
				"uninstall": {
					Input:  "chartUninstallAction",
					Output: "chartActionOutput",
				},
			}
		},
	}
	repoTemplate := schema2.Template{
		Group: catalog.GroupName,
		Kind:  "Repo",
		Customize: func(apiSchema *types.APISchema) {
			apiSchema.ActionHandlers = map[string]http.Handler{
				"install": ops,
				"upgrade": ops,
			}
			apiSchema.ResourceActions = map[string]schemas3.Action{
				"install": {
					Input:  "chartInstallAction",
					Output: "chartActionOutput",
				},
				"upgrade": {
					Input:  "chartUpgradeAction",
					Output: "chartActionOutput",
				},
			}
			apiSchema.LinkHandlers = map[string]http.Handler{
				"index": index,
				"info":  index,
				"chart": index,
				"icon":  responsewriter.ContentType(index),
			}
		},
	}
	chartRepoTemplate := repoTemplate
	chartRepoTemplate.Kind = "ClusterRepo"

	server.SchemaFactory.AddTemplate(
		operationTemplate,
		appTemplate,
		repoTemplate,
		chartRepoTemplate)
}

func isClusterRepo(typeName string) bool {
	return typeName == "catalog.cattle.io.clusterrepo"
}
