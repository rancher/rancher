package multiclustermanager

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rancher/apiserver/pkg/parse"
	"github.com/rancher/rancher/pkg/api/norman"
	"github.com/rancher/rancher/pkg/api/norman/customization/aks"
	"github.com/rancher/rancher/pkg/api/norman/customization/clusterregistrationtokens"
	"github.com/rancher/rancher/pkg/api/norman/customization/gke"
	"github.com/rancher/rancher/pkg/api/norman/customization/oci"
	"github.com/rancher/rancher/pkg/api/norman/customization/vsphere"
	managementapi "github.com/rancher/rancher/pkg/api/norman/server"
	"github.com/rancher/rancher/pkg/api/steve/supportconfigs"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/webhook"
	"github.com/rancher/rancher/pkg/channelserver"
	"github.com/rancher/rancher/pkg/clustermanager"
	rancherdialer "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/httpproxy"
	k8sProxyPkg "github.com/rancher/rancher/pkg/k8sproxy"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/multiclustermanager/whitelist"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/rkenodeconfigserver"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/tunnelserver/mcmauthorizer"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/version"
	"github.com/rancher/steve/pkg/auth"
)

func router(ctx context.Context, localClusterEnabled bool, tunnelAuthorizer *mcmauthorizer.Authorizer, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager) (func(http.Handler) http.Handler, error) {
	var (
		k8sProxy             = k8sProxyPkg.New(scaledContext, scaledContext.Dialer, clusterManager)
		connectHandler       = scaledContext.Dialer.(*rancherdialer.Factory).TunnelServer
		connectConfigHandler = rkenodeconfigserver.Handler(tunnelAuthorizer, scaledContext)
		clusterImport        = clusterregistrationtokens.ClusterImport{Clusters: scaledContext.Management.Clusters("")}
	)

	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	publicAPI, err := publicapi.NewHandler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	managementAPI, err := managementapi.New(ctx, scaledContext, clusterManager, k8sProxy, localClusterEnabled)
	if err != nil {
		return nil, err
	}

	metaProxy, err := httpproxy.NewProxy("/proxy/", whitelist.Proxy.Get, scaledContext)
	if err != nil {
		return nil, err
	}

	metricsHandler := metrics.NewMetricsHandler(scaledContext, clusterManager, promhttp.Handler())

	channelserver := channelserver.NewHandler(ctx)

	supportConfigGenerator := supportconfigs.NewHandler(scaledContext)
	// Unauthenticated routes
	unauthed := mux.NewRouter()
	unauthed.UseEncodedPath()

	unauthed.Path("/").MatcherFunc(parse.MatchNotBrowser).Handler(managementAPI)
	unauthed.Handle("/v3/connect/config", connectConfigHandler)
	unauthed.Handle("/v3/connect", connectHandler)
	unauthed.Handle("/v3/connect/register", connectHandler)
	unauthed.Handle("/v3/import/{token}_{clusterId}.yaml", http.HandlerFunc(clusterImport.ClusterImportHandler))
	unauthed.Handle("/v3/settings/cacerts", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/v3/settings/first-login", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/v3/settings/ui-banners", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/v3/settings/ui-issues", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/v3/settings/ui-pl", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/v3/settings/ui-brand", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/v3/settings/ui-default-landing", managementAPI).MatcherFunc(onlyGet)
	unauthed.Handle("/rancherversion", version.NewVersionHandler())
	unauthed.PathPrefix("/v1-{prefix}-release/channel").Handler(channelserver)
	unauthed.PathPrefix("/v1-{prefix}-release/release").Handler(channelserver)
	unauthed.PathPrefix("/v1-saml").Handler(saml.AuthHandler())
	unauthed.PathPrefix("/v3-public").Handler(publicAPI)

	// Authenticated routes
	impersonatingAuth := auth.ToMiddleware(requests.NewImpersonatingAuth(sar.NewSubjectAccessReview(clusterManager)))
	saAuth := auth.ToMiddleware(requests.NewServiceAccountAuth(scaledContext, clustermanager.ToRESTConfig))
	accessControlHandler := rbac.NewAccessControlHandler()

	saauthed := mux.NewRouter()
	saauthed.UseEncodedPath()
	saauthed.PathPrefix("/k8s/clusters/{clusterID}").Handler(k8sProxy)
	saauthed.Use(mux.MiddlewareFunc(saAuth.Chain(impersonatingAuth)))
	saauthed.Use(mux.MiddlewareFunc(accessControlHandler))
	saauthed.Use(requests.NewAuthenticatedFilter)

	authed := mux.NewRouter()
	authed.UseEncodedPath()

	authed.Use(mux.MiddlewareFunc(impersonatingAuth))
	authed.Use(mux.MiddlewareFunc(accessControlHandler))
	authed.Use(requests.NewAuthenticatedFilter)

	authed.Path("/meta/{resource:aks.+}").Handler(aks.NewAKSHandler(scaledContext))
	authed.Path("/meta/{resource:gke.+}").Handler(gke.NewGKEHandler(scaledContext))
	authed.Path("/meta/oci/{resource}").Handler(oci.NewOCIHandler(scaledContext))
	authed.Path("/meta/vsphere/{field}").Handler(vsphere.NewVsphereHandler(scaledContext))
	authed.Path("/v3/tokenreview").Methods(http.MethodPost).Handler(&webhook.TokenReviewer{})
	authed.Path("/metrics/{clusterID}").Handler(metricsHandler)
	authed.Path(supportconfigs.Endpoint).Handler(&supportConfigGenerator)
	authed.PathPrefix("/k8s/clusters/").Handler(k8sProxy)
	authed.PathPrefix("/meta/proxy").Handler(metaProxy)
	authed.PathPrefix("/v1-telemetry").Handler(telemetry.NewProxy())
	authed.PathPrefix("/v3/identit").Handler(tokenAPI)
	authed.PathPrefix("/v3/token").Handler(tokenAPI)
	authed.PathPrefix("/v3").Handler(managementAPI)

	// Metrics authenticated route
	metricsAuthed := mux.NewRouter()
	metricsAuthed.UseEncodedPath()
	tokenReviewAuth := auth.ToMiddleware(requests.NewTokenReviewAuth(scaledContext.K8sClient.AuthenticationV1()))
	metricsAuthed.Use(mux.MiddlewareFunc(tokenReviewAuth.Chain(impersonatingAuth)))
	metricsAuthed.Use(mux.MiddlewareFunc(accessControlHandler))
	metricsAuthed.Use(requests.NewAuthenticatedFilter)

	metricsAuthed.Path("/metrics").Handler(metricsHandler)

	unauthed.NotFoundHandler = saauthed
	saauthed.NotFoundHandler = authed
	authed.NotFoundHandler = metricsAuthed

	return func(next http.Handler) http.Handler {
		metricsAuthed.NotFoundHandler = next
		return unauthed
	}, nil
}

// onlyGet will match only GET but will not return a 405 like route.Methods and instead just not match
func onlyGet(req *http.Request, m *mux.RouteMatch) bool {
	return req.Method == http.MethodGet
}
