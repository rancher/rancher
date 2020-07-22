package server

import (
	"context"
	"net/http"

	"github.com/rancher/rancher/pkg/api/norman"

	"github.com/rancher/rancher/pkg/api/norman/customization/oci"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	"github.com/rancher/rancher/pkg/api/norman/customization/clusterregistrationtokens"
	"github.com/rancher/rancher/pkg/api/norman/customization/vsphere"
	managementapi "github.com/rancher/rancher/pkg/api/norman/server"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/auth/tokens"
	webhook2 "github.com/rancher/rancher/pkg/auth/webhook"
	"github.com/rancher/rancher/pkg/channelserver"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/clusterrouter"
	rancherdialer "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/httpproxy"
	k8sProxyPkg "github.com/rancher/rancher/pkg/k8sproxy"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/multiclustermanager/server/capabilities"
	"github.com/rancher/rancher/pkg/multiclustermanager/server/ui"
	"github.com/rancher/rancher/pkg/multiclustermanager/server/whitelist"
	"github.com/rancher/rancher/pkg/pipeline/hooks"
	"github.com/rancher/rancher/pkg/rkenodeconfigserver"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/websocket"
	"github.com/rancher/steve/pkg/auth"
)

func Start(ctx context.Context, localClusterEnabled bool, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager, auditLogWriter *audit.LogWriter, authz auth.Middleware) (auth.Middleware, http.Handler, error) {
	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, nil, err
	}

	publicAPI, err := publicapi.NewHandler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, nil, err
	}

	k8sProxy := k8sProxyPkg.New(scaledContext, scaledContext.Dialer)

	managementAPI, err := managementapi.New(ctx, scaledContext, clusterManager, k8sProxy, localClusterEnabled)
	if err != nil {
		return nil, nil, err
	}

	root := mux.NewRouter()
	root.UseEncodedPath()
	root.Use(responsewriter.ContentTypeOptions)

	rawAuthedAPIs := newAuthed(tokenAPI, managementAPI, k8sProxy, scaledContext)

	auth := requests.NewAuthenticator(ctx, clusterrouter.GetClusterID, scaledContext)
	auth = requests.NewImpersonatingAuth(auth, sar.NewSubjectAccessReview(clusterManager))
	if f, ok := scaledContext.Dialer.(*rancherdialer.Factory); ok {
		auth = requests.Chain(auth, f.TunnelAuthorizer)
	}

	authMiddleware := requests.ToAuthMiddleware(auth)
	tokenReview := &webhook2.TokenReviewer{
		Authenticator: auth,
	}

	authedHandler, err := requests.NewAuthenticationFilter(auth, rawAuthedAPIs)
	if err != nil {
		return nil, nil, err
	}
	authedHandler = authz.Wrap(authedHandler)

	metricsHandler, err := requests.NewAuthenticationFilter(auth, metrics.NewMetricsHandler(scaledContext, promhttp.Handler()))
	if err != nil {
		return nil, nil, err
	}

	websocketHandler := websocket.NewWebsocketHandler(authedHandler)

	auditHandler := audit.NewAuditLogFilter(ctx, auditLogWriter, websocketHandler)

	webhookHandler := hooks.New(scaledContext)

	connectHandler, connectConfigHandler := connectHandlers(scaledContext)

	samlRoot := saml.AuthHandler()
	chain := responsewriter.NewMiddlewareChain(responsewriter.Gzip, responsewriter.NoCache, responsewriter.DenyFrameOptions, responsewriter.ContentType, ui.UI)
	chainGzip := responsewriter.NewMiddlewareChain(responsewriter.Gzip, responsewriter.ContentType)

	root.Handle("/", chain.Handler(managementAPI))
	root.PathPrefix("/v3-public").Handler(publicAPI)
	root.Handle("/v3/import/{token}.yaml", http.HandlerFunc(clusterregistrationtokens.ClusterImportHandler))
	root.Handle("/v3/connect", connectHandler)
	root.Handle("/v3/connect/register", connectHandler)
	root.Handle("/v3/connect/config", connectConfigHandler)
	root.Handle("/v3/settings/cacerts", rawAuthedAPIs).Methods(http.MethodGet)
	root.Handle("/v3/settings/first-login", rawAuthedAPIs).Methods(http.MethodGet)
	root.Handle("/v3/settings/ui-pl", rawAuthedAPIs).Methods(http.MethodGet)
	root.Handle("/v3/settings/ui-banners", rawAuthedAPIs).Methods(http.MethodGet)
	root.Handle("/v3/settings/ui-issues", rawAuthedAPIs).Methods(http.MethodGet)
	root.Handle("/v3/tokenreview", tokenReview).Methods(http.MethodPost)
	root.PathPrefix("/metrics").Handler(metricsHandler)
	root.PathPrefix("/v3").Handler(chainGzip.Handler(auditHandler))
	root.PathPrefix("/hooks").Handler(webhookHandler)
	root.PathPrefix("/k8s/clusters/").Handler(auditHandler)
	root.PathPrefix("/meta").Handler(auditHandler)
	root.PathPrefix("/v1-telemetry").Handler(auditHandler)
	root.PathPrefix("/v1-release/release").Handler(channelserver.NewProxy(ctx))
	root.NotFoundHandler = ui.UI(http.NotFoundHandler())
	root.PathPrefix("/v1-saml").Handler(samlRoot)

	// UI
	uiContent := responsewriter.NewMiddlewareChain(responsewriter.Gzip, responsewriter.DenyFrameOptions, responsewriter.CacheMiddleware("json", "js", "css")).Handler(ui.Content())
	root.PathPrefix("/assets").Handler(uiContent)
	root.PathPrefix("/translations").Handler(uiContent)
	root.PathPrefix("/ember-fetch").Handler(uiContent)
	root.PathPrefix("/engines-dist").Handler(uiContent)
	root.Handle("/asset-manifest.json", uiContent)
	root.Handle("/crossdomain.xml", uiContent)
	root.Handle("/humans.txt", uiContent)
	root.Handle("/index.html", uiContent)
	root.Handle("/robots.txt", uiContent)
	root.Handle("/VERSION.txt", uiContent)

	//API UI
	root.PathPrefix("/api-ui").Handler(uiContent)

	registerHealth(root)

	return authMiddleware, root, err
}

func newAuthed(tokenAPI http.Handler, managementAPI http.Handler, k8sproxy http.Handler, scaledContext *config.ScaledContext) *mux.Router {
	authed := mux.NewRouter()
	authed.UseEncodedPath()
	authed.Use(responsewriter.ContentTypeOptions)
	authed.Path("/meta/gkeMachineTypes").Handler(capabilities.NewGKEMachineTypesHandler())
	authed.Path("/meta/gkeVersions").Handler(capabilities.NewGKEVersionsHandler())
	authed.Path("/meta/gkeZones").Handler(capabilities.NewGKEZonesHandler())
	authed.Path("/meta/gkeNetworks").Handler(capabilities.NewGKENetworksHandler())
	authed.Path("/meta/gkeSubnetworks").Handler(capabilities.NewGKESubnetworksHandler())
	authed.Path("/meta/gkeServiceAccounts").Handler(capabilities.NewGKEServiceAccountsHandler())
	authed.Path("/meta/aksVersions").Handler(capabilities.NewAKSVersionsHandler())
	authed.Path("/meta/aksVirtualNetworks").Handler(capabilities.NewAKSVirtualNetworksHandler())
	authed.Path("/meta/oci/{resource}").Handler(oci.NewOCIHandler(scaledContext))
	authed.Path("/meta/vsphere/{field}").Handler(vsphere.NewVsphereHandler(scaledContext))
	authed.PathPrefix("/meta/proxy").Handler(newProxy(scaledContext))
	authed.PathPrefix("/meta").Handler(managementAPI)
	authed.PathPrefix("/v3/identit").Handler(tokenAPI)
	authed.PathPrefix("/v3/token").Handler(tokenAPI)
	authed.PathPrefix("/k8s/clusters/").Handler(k8sproxy)
	authed.PathPrefix("/v1-telemetry").Handler(telemetry.NewProxy())
	authed.PathPrefix(managementSchema.Version.Path).Handler(managementAPI)

	return authed
}

func connectHandlers(scaledContext *config.ScaledContext) (http.Handler, http.Handler) {
	if f, ok := scaledContext.Dialer.(*rancherdialer.Factory); ok {
		return f.TunnelServer, rkenodeconfigserver.Handler(f.TunnelAuthorizer, scaledContext)
	}

	return http.NotFoundHandler(), http.NotFoundHandler()
}

func newProxy(scaledContext *config.ScaledContext) http.Handler {
	return httpproxy.NewProxy("/proxy/", whitelist.Proxy.Get, scaledContext)
}
