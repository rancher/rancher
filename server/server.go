package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/api/customization/clusterregistrationtokens"
	managementapi "github.com/rancher/rancher/pkg/api/server"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	authrequests "github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/hooks"
	rancherdialer "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/dynamiclistener"
	"github.com/rancher/rancher/pkg/httpproxy"
	k8sProxy "github.com/rancher/rancher/pkg/k8sproxy"
	"github.com/rancher/rancher/pkg/rkenodeconfigserver"
	"github.com/rancher/rancher/server/capabilities"
	"github.com/rancher/rancher/server/ui"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
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

func Start(ctx context.Context, httpPort, httpsPort int, apiContext *config.ScaledContext, clusterManager *clustermanager.Manager) error {
	tokenAPI, err := tokens.NewAPIHandler(ctx, apiContext)
	if err != nil {
		return err
	}

	publicAPI, err := publicapi.NewHandler(ctx, apiContext)
	if err != nil {
		return err
	}

	managementAPI, err := managementapi.New(ctx, apiContext, clusterManager)
	if err != nil {
		return err
	}

	root := mux.NewRouter()
	root.UseEncodedPath()

	app.DefaultProxyDialer = utilnet.DialFunc(apiContext.Dialer.LocalClusterDialer())

	localClusterAuth, err := k8sProxy.NewLocalProxy(apiContext, apiContext.Dialer, root)
	if err != nil {
		return err
	}

	k8sProxy := k8sProxy.New(apiContext, apiContext.Dialer)

	rawAuthedAPIs := newAuthed(tokenAPI, managementAPI, k8sProxy)

	authedHandler, err := authrequests.NewAuthenticationFilter(ctx, apiContext, rawAuthedAPIs)
	if err != nil {
		return err
	}

	webhookHandler := hooks.New(apiContext)

	connectHandler, connectConfigHandler := connectHandlers(apiContext.Dialer)

	root.Handle("/", ui.UI(managementAPI))
	root.PathPrefix("/v3-public").Handler(publicAPI)
	root.Handle("/v3/import/{token}.yaml", http.HandlerFunc(clusterregistrationtokens.ClusterImportHandler))
	root.Handle("/v3/connect", connectHandler)
	root.Handle("/v3/connect/register", connectHandler)
	root.Handle("/v3/connect/config", connectConfigHandler)
	root.Handle("/v3/settings/cacerts", rawAuthedAPIs).Methods(http.MethodGet)
	root.PathPrefix("/v3").Handler(authedHandler)
	root.PathPrefix("/hooks").Handler(webhookHandler)
	root.PathPrefix("/k8s/clusters/").Handler(authedHandler)
	root.PathPrefix("/meta").Handler(authedHandler)
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

	dynamiclistener.Start(ctx, apiContext, httpPort, httpsPort, localClusterAuth)
	return nil
}

func newAuthed(tokenAPI http.Handler, managementAPI http.Handler, k8sproxy http.Handler) *mux.Router {
	authed := mux.NewRouter()
	authed.UseEncodedPath()
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

func connectHandlers(dialer dialer.Factory) (http.Handler, http.Handler) {
	if f, ok := dialer.(*rancherdialer.Factory); ok {
		return f.TunnelServer, rkenodeconfigserver.Handler(f.TunnelAuthorizer)
	}

	return http.NotFoundHandler(), http.NotFoundHandler()
}

func newProxy() http.Handler {
	return httpproxy.NewProxy("/proxy/", func() []string {
		return whiteList
	})
}
