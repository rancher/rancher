package scim

import (
	"net/http"

	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
)

// SCIMServer handles SCIM API requests.
type SCIMServer struct {
	groups             v3.GroupClient
	groupsCache        v3.GroupCache
	userCache          v3.UserCache
	users              v3.UserClient
	userAttributeCache v3.UserAttributeCache
	userAttributes     v3.UserAttributeClient
	userMGR            user.Manager
	getConfig          func(provider string) providerConfig
}

// NewHandler instantiates [SCIMServer] and returns an [http.Handler] that serves SCIM API endpoints.
func NewHandler(scaledContext *config.ScaledContext) http.Handler {
	configMapCache := scaledContext.Wrangler.Core.ConfigMap().Cache()
	srv := &SCIMServer{
		groups:             scaledContext.Wrangler.Mgmt.Group(),
		groupsCache:        scaledContext.Wrangler.Mgmt.Group().Cache(),
		userCache:          scaledContext.Wrangler.Mgmt.User().Cache(),
		users:              scaledContext.Wrangler.Mgmt.User(),
		userAttributeCache: scaledContext.Wrangler.Mgmt.UserAttribute().Cache(),
		userAttributes:     scaledContext.Wrangler.Mgmt.UserAttribute(),
		userMGR:            scaledContext.UserManager,
		getConfig: func(provider string) providerConfig {
			return getProviderConfig(configMapCache, provider)
		},
	}

	authenticator := NewTokenAuthenticator(scaledContext.Wrangler)

	r := http.NewServeMux()

	// Setup the middleware
	authWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return authenticator.Authenticate(h).ServeHTTP
	}

	// Discovery endpoints
	r.HandleFunc("GET "+URLPrefix+"/{provider}/ServiceProviderConfig", authWrap(srv.GetServiceProviderConfig))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/ResourceTypes", authWrap(srv.ListResourceTypes))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/ResourceTypes/{id}", authWrap(srv.GetResourceType))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Schemas", authWrap(srv.ListSchemas))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Schemas/{id}", authWrap(srv.GetSchema))

	// User endpoints
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Users", authWrap(srv.ListUsers))
	r.HandleFunc("POST "+URLPrefix+"/{provider}/Users", authWrap(srv.CreateUser))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Users/{id}", authWrap(srv.GetUser))
	r.HandleFunc("PUT "+URLPrefix+"/{provider}/Users/{id}", authWrap(srv.UpdateUser))
	r.HandleFunc("PATCH "+URLPrefix+"/{provider}/Users/{id}", authWrap(srv.PatchUser))
	r.HandleFunc("DELETE "+URLPrefix+"/{provider}/Users/{id}", authWrap(srv.DeleteUser))

	// Group endpoints
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Groups", authWrap(srv.ListGroups))
	r.HandleFunc("POST "+URLPrefix+"/{provider}/Groups", authWrap(srv.CreateGroup))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Groups/{id}", authWrap(srv.GetGroup))
	r.HandleFunc("PUT "+URLPrefix+"/{provider}/Groups/{id}", authWrap(srv.UpdateGroup))
	r.HandleFunc("PATCH "+URLPrefix+"/{provider}/Groups/{id}", authWrap(srv.PatchGroup))
	r.HandleFunc("DELETE "+URLPrefix+"/{provider}/Groups/{id}", authWrap(srv.DeleteGroup))

	return r
}
