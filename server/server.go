package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/auth/filter"
	"github.com/rancher/auth/tokens"
	managementapi "github.com/rancher/management-api/server"
	"github.com/rancher/rancher/server/proxy"
	"github.com/rancher/rancher/server/ui"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

var (
	whiteList = []string{
		"*.amazonaws.com",
		"*.amazonaws.com.cn",
		"forums.rancher.com",
		"api.exoscale.ch",
		"api.ubiquityhosting.com",
		"api.digitalocean.com",
		"*.otc.t-systems.com",
		"api.profitbricks.com",
	}
)

func New(ctx context.Context, management *config.ManagementContext) (http.Handler, error) {
	tokenAPI, err := tokens.NewTokenAPIHandler(ctx, management)

	managementAPI, err := managementapi.New(ctx, management)
	if err != nil {
		return nil, err
	}

	authedHandler, err := filter.NewAuthenticationFilter(ctx, management, newAuthed(tokenAPI, managementAPI))
	if err != nil {
		return nil, err
	}

	unauthed := mux.NewRouter()
	unauthed.Handle("/", ui.UI(managementAPI))
	unauthed.PathPrefix("/v3/token").Queries("action", "login").Handler(tokenAPI)
	unauthed.PathPrefix("/v3/token").Queries("action", "logout").Handler(tokenAPI)
	unauthed.NotFoundHandler = ui.UI(http.NotFoundHandler())
	unauthed.PathPrefix("/v3").Handler(authedHandler)
	unauthed.PathPrefix("/meta").Handler(authedHandler)

	return unauthed, nil
}

func newAuthed(tokenAPI http.Handler, managementAPI http.Handler) *mux.Router {
	authed := mux.NewRouter()
	authed.PathPrefix("/meta/proxy").Handler(newProxy())
	authed.PathPrefix("/v3/token").Handler(tokenAPI)
	authed.PathPrefix("/v3/identit").Handler(tokenAPI)
	authed.PathPrefix("/meta").Handler(managementAPI)
	authed.PathPrefix(managementSchema.Version.Path).Handler(managementAPI)
	return authed
}

func newProxy() http.Handler {
	return proxy.NewProxy("/proxy/", func() []string {
		return whiteList
	})
}
