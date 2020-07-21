package catalog

import (
	"context"
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/apis/catalog.cattle.io"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	v12 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	schemas3 "github.com/rancher/wrangler/pkg/schemas"
)

func Register(ctx context.Context, server *steve.Server,
	secrets v12.SecretClient,
	pods v12.PodClient,
	configMaps v12.ConfigMapClient,
	catalog catalogcontrollers.Interface) error {

	ops := newOperation(server.ClientFactory, catalog, pods, secrets)
	server.ClusterCache.OnAdd(ctx, ops.OnAdd)
	server.ClusterCache.OnChange(ctx, ops.OnChange)

	index := &indexDownload{
		configMaps:   configMaps,
		repos:        catalog.Repo(),
		clusterRepos: catalog.ClusterRepo(),
	}

	addSchemas(server, ops, index)
	return nil
}

func addSchemas(server *steve.Server, ops *operation, index http.Handler) {
	server.BaseSchemas.MustImportAndCustomize(v1.ChartUninstallAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(v1.ChartUpgradeAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(v1.ChartRollbackAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(v1.ChartInstallAction{}, nil)
	server.BaseSchemas.MustImportAndCustomize(v1.ChartActionOutput{}, nil)

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
	releaseTemplate := schema2.Template{
		Group: catalog.GroupName,
		Kind:  "Release",
		Customize: func(apiSchema *types.APISchema) {
			apiSchema.ActionHandlers = map[string]http.Handler{
				"rollback":  ops,
				"uninstall": ops,
			}
			apiSchema.ResourceActions = map[string]schemas3.Action{
				"rollback": {
					Input:  "chartRollbackAction",
					Output: "chartActionOutput",
				},
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
			}
		},
	}
	chartRepoTemplate := repoTemplate
	chartRepoTemplate.Kind = "ClusterRepo"

	server.SchemaTemplates = append(server.SchemaTemplates,
		operationTemplate,
		releaseTemplate,
		repoTemplate,
		chartRepoTemplate)
}

func isClusterRepo(typeName string) bool {
	return typeName == "catalog.cattle.io.clusterrepo"
}
