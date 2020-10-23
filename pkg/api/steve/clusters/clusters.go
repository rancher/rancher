package clusters

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/handlers"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/podimpersonation"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Register(ctx context.Context, server *steve.Server) error {
	shell := &shell{
		cg:           server.ClientFactory,
		namespace:    "cattle-system",
		impersonator: podimpersonation.New("shell", server.ClientFactory, time.Hour, settings.FullShellImage),
	}

	server.ClusterCache.OnAdd(ctx, shell.impersonator.PurgeOldRoles)
	server.ClusterCache.OnChange(ctx, func(gvr schema.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
		return shell.impersonator.PurgeOldRoles(gvr, key, obj)
	})

	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "management.cattle.io",
		Kind:  "Cluster",
		Customize: func(schema *types.APISchema) {
			schema.LinkHandlers = map[string]http.Handler{
				"shell": shell,
			}
			schema.ByIDHandler = func(request *types.APIRequest) (types.APIObject, error) {
				// By pass authorization for local shell because the user might not have
				// GET granted for local cluster
				if request.Name == "local" && request.Link == "shell" {
					shell.ServeHTTP(request.Response, request.Request)
					return types.APIObject{}, validation.ErrComplete
				}
				return handlers.ByIDHandler(request)
			}
			// Everybody can list even if they have no list or get privileges. The users
			// authorization will still be used to determine what can be seen but just
			// may result in an empty list
			schema.CollectionMethods = append(schema.CollectionMethods, http.MethodGet)
		},
	})
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "management.cattle.io",
		Kind:  "Project",
		Customize: func(schema *types.APISchema) {
			// Everybody can list even if they have no list or get privileges. The users
			// authorization will still be used to determine what can be seen but just
			// may result in an empty list
			schema.CollectionMethods = append(schema.CollectionMethods, http.MethodGet)
		},
	})
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "",
		Kind:  "Namespace",
		Customize: func(schema *types.APISchema) {
			// Everybody can list even if they have no list or get privileges. The users
			// authorization will still be used to determine what can be seen but just
			// may result in an empty list
			schema.CollectionMethods = append(schema.CollectionMethods, http.MethodGet)
		},
	})

	return nil
}
