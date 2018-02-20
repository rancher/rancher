package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	k8sProxy "github.com/rancher/rancher/k8s/proxy"
	"github.com/rancher/rancher/pkg/api/customization/clusteregistrationtokens"
	managementapi "github.com/rancher/rancher/pkg/api/server"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	authrequests "github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/dynamiclistener"
	"github.com/rancher/rancher/pkg/httpproxy"
	"github.com/rancher/rancher/server/capabilities"
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
		"api.packet.net",
	}
)

func Start(ctx context.Context, httpPort, httpsPort int, apiContext *config.ScaledContext) error {
	tokenAPI, err := tokens.NewAPIHandler(ctx, apiContext)
	if err != nil {
		return err
	}

	publicAPI, err := publicapi.NewHandler(ctx, apiContext)
	if err != nil {
		return err
	}

	managementAPI, err := managementapi.New(ctx, apiContext)
	if err != nil {
		return err
	}

	k8sProxy := k8sProxy.New(apiContext, apiContext.Dialer)

	authedAPIs := newAuthed(tokenAPI, managementAPI, k8sProxy)

	authedHandler, err := authrequests.NewAuthenticationFilter(ctx, apiContext, authedAPIs)
	if err != nil {
		return err
	}

	root := mux.NewRouter()
	root.Handle("/", ui.UI(managementAPI))
	root.Handle("/v3/settings/cacerts", authedAPIs).Methods(http.MethodGet)
	root.PathPrefix("/v3-public").Handler(publicAPI)
	root.Handle("/v3/import/{token}.yaml", http.HandlerFunc(clusteregistrationtokens.ClusterImportHandler))
	if f, ok := apiContext.Dialer.(*dialer.Factory); ok {
		root.PathPrefix("/v3/connect").Handler(f.TunnelServer)
	}
	root.PathPrefix("/v3").Handler(authedHandler)
	root.PathPrefix("/meta").Handler(authedHandler)
	root.PathPrefix("/k8s/clusters/").Handler(authedHandler)
	root.NotFoundHandler = ui.UI(http.NotFoundHandler())

	// UI
	uiContent := ui.Content()
	root.PathPrefix("/assets").Handler(uiContent)
	root.PathPrefix("/translations").Handler(uiContent)
	root.Handle("/humans.txt", uiContent)
	root.Handle("/index.html", uiContent)
	root.Handle("/robots.txt", uiContent)
	root.Handle("/VERSION.txt", uiContent)

	registerHealth(root)

	dynamiclistener.Start(ctx, apiContext, httpPort, httpsPort, root)
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
	return httpproxy.NewProxy("/proxy/", func() []string {
		return whiteList
	})
}
