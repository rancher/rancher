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
	rateLimiter := newRateLimiter(func(provider string) (int, int) {
		cfg := srv.getConfig(provider)
		return cfg.RateLimitRequestsPerSecond, cfg.RateLimitBurst
	})

	r := http.NewServeMux()

	middlewares := func(h http.HandlerFunc) http.HandlerFunc {
		return authenticator.Authenticate(rateLimiter.Wrap(h)).ServeHTTP
	}

	// Discovery endpoints
	r.HandleFunc("GET "+URLPrefix+"/{provider}/ServiceProviderConfig", middlewares(srv.GetServiceProviderConfig))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/ResourceTypes", middlewares(srv.ListResourceTypes))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/ResourceTypes/{id}", middlewares(srv.GetResourceType))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Schemas", middlewares(srv.ListSchemas))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Schemas/{id}", middlewares(srv.GetSchema))

	// User endpoints
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Users", middlewares(srv.ListUsers))
	r.HandleFunc("POST "+URLPrefix+"/{provider}/Users", middlewares(srv.CreateUser))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Users/{id}", middlewares(srv.GetUser))
	r.HandleFunc("PUT "+URLPrefix+"/{provider}/Users/{id}", middlewares(srv.UpdateUser))
	r.HandleFunc("PATCH "+URLPrefix+"/{provider}/Users/{id}", middlewares(srv.PatchUser))
	r.HandleFunc("DELETE "+URLPrefix+"/{provider}/Users/{id}", middlewares(srv.DeleteUser))

	// Group endpoints
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Groups", middlewares(srv.ListGroups))
	r.HandleFunc("POST "+URLPrefix+"/{provider}/Groups", middlewares(srv.CreateGroup))
	r.HandleFunc("GET "+URLPrefix+"/{provider}/Groups/{id}", middlewares(srv.GetGroup))
	r.HandleFunc("PUT "+URLPrefix+"/{provider}/Groups/{id}", middlewares(srv.UpdateGroup))
	r.HandleFunc("PATCH "+URLPrefix+"/{provider}/Groups/{id}", middlewares(srv.PatchGroup))
	r.HandleFunc("DELETE "+URLPrefix+"/{provider}/Groups/{id}", middlewares(srv.DeleteGroup))

	return r
}
