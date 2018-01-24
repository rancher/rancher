package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/auth/filter"
	"github.com/rancher/auth/server"
	managementapi "github.com/rancher/management-api/server"
	k8sProxy "github.com/rancher/rancher/k8s/proxy"
	"github.com/rancher/rancher/server/capabilities"
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

func New(ctx context.Context, httpPort, httpsPort int, management *config.ManagementContext) error {
	var result http.Handler
	tokenAPI, err := server.NewTokenAPIHandler(ctx, management)

	managementAPI, err := managementapi.New(ctx, httpPort, httpsPort, management, func() http.Handler {
		return result
	})

	if err != nil {
		return err
	}

	k8sProxy, err := k8sProxy.New(management)
	if err != nil {
		return err
	}

	authedHandler, err := filter.NewAuthenticationFilter(ctx, management,
		newAuthed(tokenAPI, managementAPI, k8sProxy))
	if err != nil {
		return err
	}

	unauthed := mux.NewRouter()
	unauthed.Handle("/", ui.UI(managementAPI))
	unauthed.PathPrefix("/v3/token").Handler(tokenAPI)
	unauthed.NotFoundHandler = ui.UI(http.NotFoundHandler())
	unauthed.PathPrefix("/v3").Handler(authedHandler)
	unauthed.PathPrefix("/meta").Handler(authedHandler)
	unauthed.PathPrefix("/k8s/clusters/").Handler(authedHandler)

	uiContent := ui.Content()
	unauthed.PathPrefix("/assets").Handler(uiContent)
	unauthed.PathPrefix("/translations").Handler(uiContent)
	unauthed.Handle("humans.txt", uiContent)
	unauthed.Handle("index.html", uiContent)
	unauthed.Handle("robots.txt", uiContent)
	unauthed.Handle("VERSION.txt", uiContent)

	registerHealth(unauthed)

	result = unauthed
	return nil
}

func newAuthed(tokenAPI http.Handler, managementAPI http.Handler, k8sproxy http.Handler) *mux.Router {
	authed := mux.NewRouter()
	authed.PathPrefix("/meta/proxy").Handler(newProxy())
	authed.PathPrefix("/meta").Handler(managementAPI)
	authed.PathPrefix("/v3/gkeMachineTypes").Handler(capabilities.NewGKEMachineTypesHandler())
	authed.PathPrefix("/v3/gkeVersions").Handler(capabilities.NewGKEVersionsHandler())
	authed.PathPrefix("/v3/gkeZones").Handler(capabilities.NewGKEZonesHandler())
	authed.PathPrefix("/v3/identit").Handler(tokenAPI)
	authed.PathPrefix("/v3/token").Handler(tokenAPI)
	authed.PathPrefix("/k8s/clusters/").Handler(k8sproxy)
	authed.PathPrefix(managementSchema.Version.Path).Handler(managementAPI)

	return authed
}

func newProxy() http.Handler {
	return proxy.NewProxy("/proxy/", func() []string {
		return whiteList
	})
}
