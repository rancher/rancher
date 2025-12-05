package scim

import (
	"net/http"

	"github.com/gorilla/mux"
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
}

// NewHandler instantiates [SCIMServer] and returns an [http.Handler] that serves SCIM API endpoints.
func NewHandler(scaledContext *config.ScaledContext) http.Handler {
	srv := &SCIMServer{
		groups:             scaledContext.Wrangler.Mgmt.Group(),
		groupsCache:        scaledContext.Wrangler.Mgmt.Group().Cache(),
		userCache:          scaledContext.Wrangler.Mgmt.User().Cache(),
		users:              scaledContext.Wrangler.Mgmt.User(),
		userAttributeCache: scaledContext.Wrangler.Mgmt.UserAttribute().Cache(),
		userAttributes:     scaledContext.Wrangler.Mgmt.UserAttribute(),
		userMGR:            scaledContext.UserManager,
	}

	authenticator := NewTokenAuthenticator(scaledContext.Wrangler.Core.Secret().Cache())

	r := mux.NewRouter().UseEncodedPath().StrictSlash(true)
	r.Use(authenticator.Authenticate)
	// Discovery.
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/ServiceProviderConfig").HandlerFunc(srv.GetServiceProviderConfig)
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/ResourceTypes").HandlerFunc(srv.ListResourceTypes)
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/ResourceTypes/{id}").HandlerFunc(srv.GetResourceType)
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/Schemas").HandlerFunc(srv.ListSchemas)
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/Schemas/{id}").HandlerFunc(srv.GetSchema)
	// Users.
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/Users").HandlerFunc(srv.ListUsers)
	r.Methods(http.MethodPost).Path(URLPrefix + "/{provider}/Users").HandlerFunc(srv.CreateUser)
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/Users/{id}").HandlerFunc(srv.GetUser)
	r.Methods(http.MethodPut).Path(URLPrefix + "/{provider}/Users/{id}").HandlerFunc(srv.UpdateUser)
	r.Methods(http.MethodPatch).Path(URLPrefix + "/{provider}/Users/{id}").HandlerFunc(srv.PatchUser)
	r.Methods(http.MethodDelete).Path(URLPrefix + "/{provider}/Users/{id}").HandlerFunc(srv.DeleteUser)
	// Groups.
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/Groups").HandlerFunc(srv.ListGroups)
	r.Methods(http.MethodPost).Path(URLPrefix + "/{provider}/Groups").HandlerFunc(srv.CreateGroup)
	r.Methods(http.MethodGet).Path(URLPrefix + "/{provider}/Groups/{id}").HandlerFunc(srv.GetGroup)
	r.Methods(http.MethodPut).Path(URLPrefix + "/{provider}/Groups/{id}").HandlerFunc(srv.UpdateGroup)
	r.Methods(http.MethodPatch).Path(URLPrefix + "/{provider}/Groups/{id}").HandlerFunc(srv.PatchGroup)
	r.Methods(http.MethodDelete).Path(URLPrefix + "/{provider}/Groups/{id}").HandlerFunc(srv.DeleteGroup)

	return r
}
