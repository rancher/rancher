package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	k8sProxy "github.com/rancher/rancher/k8s/proxy"
	managementapi "github.com/rancher/rancher/pkg/api/management/server"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	authrequests "github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/tunnel"
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

type Server struct {
	Tunneler *remotedialer.Server
}

func New(ctx context.Context, httpPort, httpsPort int, management *config.ManagementContext) (*Server, error) {
	var result http.Handler
	tokenAPI, err := tokens.NewAPIHandler(ctx, management)
	if err != nil {
		return nil, err
	}

	publicAPI, err := publicapi.NewHandler(ctx, management)
	if err != nil {
		return nil, err
	}

	managementAPI, err := managementapi.New(ctx, httpPort, httpsPort, management, func() http.Handler {
		return result
	})

	if err != nil {
		return nil, err
	}

	k8sProxy, err := k8sProxy.New(management)
	if err != nil {
		return nil, err
	}

	tunnel := tunnel.NewTunneler(management)

	authedAPIs := newAuthed(tokenAPI, managementAPI, k8sProxy)

	authedHandler, err := authrequests.NewAuthenticationFilter(ctx, management, authedAPIs)
	if err != nil {
		return nil, err
	}

	unauthed := mux.NewRouter()
	unauthed.Handle("/", ui.UI(managementAPI))
	unauthed.Handle("/v3/settings/cacerts", authedAPIs).Methods(http.MethodGet)
	unauthed.PathPrefix("/v3-public").Handler(publicAPI)
	unauthed.NotFoundHandler = ui.UI(http.NotFoundHandler())
	unauthed.PathPrefix("/v3/connect").Handler(tunnel)
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
	return &Server{
		Tunneler: tunnel,
	}, nil
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
