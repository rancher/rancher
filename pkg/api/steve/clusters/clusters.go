package clusters

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/podimpersonation"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Register(ctx context.Context, server *steve.Server) error {
	shell := &shell{
		cg:           server.ClientFactory,
		namespace:    "dashboard-shells",
		impersonator: podimpersonation.New("shell", server.ClientFactory, time.Hour, settings.FullShellImage),
	}

	server.ClusterCache.OnAdd(ctx, shell.impersonator.PurgeOldRoles)
	server.ClusterCache.OnChange(ctx, func(gvr schema.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
		return shell.impersonator.PurgeOldRoles(gvr, key, obj)
	})

	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "management.cattle.io",
		Kind:  "Cluster",
		ID:    "",
		Customize: func(schema *types.APISchema) {
			schema.LinkHandlers = map[string]http.Handler{
				"shell": shell,
			}
		},
	})

	return nil
}
