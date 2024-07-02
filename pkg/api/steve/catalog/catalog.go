/*
Package catalog implements API handlers for Rancher's catalog functionality.

It registers handlers for Helm-related operations and content management.

It also links the custom resouces with the handlers with the help of Templates in the apiserver package.

The package is used to facilitate interactions with Helm charts within a Rancher server environment.
*/
package catalog

import (
	"context"
	"net/http"

	"github.com/rancher/apiserver/pkg/handlers"
	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	"github.com/rancher/apiserver/pkg/types"
	types2 "github.com/rancher/rancher/pkg/api/steve/catalog/types"
	"github.com/rancher/rancher/pkg/apis/catalog.cattle.io"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	schemas3 "github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
)

// Register is used to register the two handlers with the apiserver
func Register(ctx context.Context, server *steve.Server,
	helmop *helmop.Operations,
	contentManager *content.Manager) error {

	ops := newOperation(helmop, server.ClusterRegistry)

	// Informer callbacks for Steve server
	server.ClusterCache.OnAdd(ctx, ops.OnAdd)
	server.ClusterCache.OnChange(ctx, ops.OnChange)

	// App's & marketplace-related data
	index := &contentDownload{
		contentManager: contentManager,
	}

	addSchemas(server, ops, index)
	return nil
}

// addSchemas adds and customizes API schemas for operations, app, repo, and clusterrepo.
// It adds action handlers and resource actions for install, upgrade, and uninstall operations of Charts.
// It also sets up handlers for byID and link requests.
//
// The function uses predefined structure templates for API schemas, allowing for customization
// of behavior at runtime. It associates specific operations with specific routes, and
// defines how to handle different action requests made on different resources.
//
// The handlers for retrieving resources by their IDs are also customized.
func addSchemas(server *steve.Server, ops *operation, index http.Handler) {
	// Imports and generates API schemas to be handled by as requests by the Rancher API server.
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
			// Customize the handler for retrieving a Repo resource by its ID.
			apiSchema.ByIDHandler = func(request *types.APIRequest) (types.APIObject, error) {
				if request.Name == "index.yaml" {
					request.Name = request.Namespace
					request.Namespace = ""
					request.Link = "index"
					// Serve the HTTP response using the 'index' handler.
					index.ServeHTTP(request.Response, request.Request)
					// The request has been fully handled and no further processing is required
					return types.APIObject{}, validation.ErrComplete
				}
				// For all other requests, use default ByIDHandler to retrieve a resource by its ID.
				return handlers.ByIDHandler(request)
			}
			// Define handlers for different links on the Repo resource that can be used to serve additional information.
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
