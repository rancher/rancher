package steve

import (
	"context"
	"net/http"
	"path"

	apitypes "github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/api/steve/catalog"
	"github.com/rancher/rancher/pkg/api/steve/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/api/steve/clusters"
	"github.com/rancher/rancher/pkg/api/steve/disallow"
	"github.com/rancher/rancher/pkg/api/steve/machine"
	"github.com/rancher/rancher/pkg/api/steve/navlinks"
	"github.com/rancher/rancher/pkg/api/steve/settings"
	"github.com/rancher/rancher/pkg/api/steve/userpreferences"
	"github.com/rancher/rancher/pkg/auth/impersonation"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
)

func Setup(ctx context.Context, server *steve.Server, wranglerContext *wrangler.Context, scaledContext *config.ScaledContext, userManager user.Manager) error {
	userpreferences.Register(server.BaseSchemas, server.ClientFactory)
	if err := clusters.Register(ctx, server, wranglerContext, userManager); err != nil {
		return err
	}
	machine.Register(server, wranglerContext)
	clusterregistrationtoken.Register(server, wranglerContext)
	navlinks.Register(ctx, server)
	settings.Register(server)
	disallow.Register(server)
	registerUserImpersonation(server, scaledContext)
	return catalog.Register(ctx,
		server,
		wranglerContext.HelmOperations,
		wranglerContext.CatalogContentManager)
}

func registerUserImpersonation(server *steve.Server, scaledContext *config.ScaledContext) {
	handler := impersonation.NewHandler(scaledContext)
	actionHandler := userImpersonationActionHandler(handler)

	server.SchemaFactory.AddTemplate(schema2.Template{
		Group:     "management.cattle.io",
		Kind:      "User",
		Formatter: userImpersonationFormatter(handler),
		Customize: func(schema *apitypes.APISchema) {
			if schema.ActionHandlers == nil {
				schema.ActionHandlers = map[string]http.Handler{}
			}
			schema.ActionHandlers["impersonate"] = actionHandler
		},
	})
}

func userImpersonationActionHandler(handler *impersonation.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		handler.StartForUser(response, request, path.Base(request.URL.Path))
	})
}

func userImpersonationFormatter(handler *impersonation.Handler) func(*apitypes.APIRequest, *apitypes.RawResource) {
	return func(apiRequest *apitypes.APIRequest, resource *apitypes.RawResource) {
		if apiRequest == nil || apiRequest.Request == nil {
			return
		}
		eligible, err := handler.CanImpersonateRequest(apiRequest.Request, resource.ID)
		if err == nil && eligible {
			resource.AddAction(apiRequest, "impersonate")
		}
	}
}
