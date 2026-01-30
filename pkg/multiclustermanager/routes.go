package multiclustermanager

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rancher/apiserver/pkg/parse"
	"github.com/rancher/rancher/pkg/api/norman"
	"github.com/rancher/rancher/pkg/api/norman/customization/aks"
	"github.com/rancher/rancher/pkg/api/norman/customization/alibaba"
	"github.com/rancher/rancher/pkg/api/norman/customization/clusterregistrationtokens"
	"github.com/rancher/rancher/pkg/api/norman/customization/gke"
	"github.com/rancher/rancher/pkg/api/norman/customization/oci"
	"github.com/rancher/rancher/pkg/api/norman/customization/vsphere"
	managementapi "github.com/rancher/rancher/pkg/api/norman/server"
	"github.com/rancher/rancher/pkg/api/steve/supportconfigs"
	"github.com/rancher/rancher/pkg/auth/logout"
	"github.com/rancher/rancher/pkg/auth/providers/publicapi"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/webhook"
	"github.com/rancher/rancher/pkg/channelserver"
	"github.com/rancher/rancher/pkg/clustermanager"
	rancherdialer "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/httpproxy"
	k8sProxyPkg "github.com/rancher/rancher/pkg/k8sproxy"
	"github.com/rancher/rancher/pkg/metrics"
	"github.com/rancher/rancher/pkg/multiclustermanager/whitelist"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/utils"
	"github.com/rancher/rancher/pkg/version"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
)

func router(ctx context.Context, localClusterEnabled bool, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager) (func(http.Handler) http.Handler, error) {
	var (
		k8sProxy       = k8sProxyPkg.New(scaledContext, scaledContext.Dialer, clusterManager)
		connectHandler = scaledContext.Dialer.(*rancherdialer.Factory).TunnelServer
		clusterImport  = clusterregistrationtokens.ClusterImport{Clusters: scaledContext.Management.Clusters("")}
	)

	logout := logout.NewHandler(ctx, tokens.NewManager(scaledContext.Wrangler))

	tokenAPI, err := tokens.NewAPIHandler(ctx, scaledContext.Wrangler, logout, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	v3PublicAPI, err := publicapi.NewV3Handler(ctx, scaledContext, norman.ConfigureAPIUI)
	if err != nil {
		return nil, err
	}

	v1PublicAPI, err := publicapi.NewV1Handler(ctx, scaledContext)
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

	channelserver := channelserver.NewHandler(ctx)

	supportConfigGenerator := supportconfigs.NewHandler(scaledContext)
	// Unauthenticated routes
	unauthed := mux.NewRouter()
	unauthed.UseEncodedPath()

	publicLimit, err := settings.APIBodyLimit.GetQuantityAsInt64(1024 * 1024)
	if err != nil {
		return nil, fmt.Errorf("parsing the public API body limit: %w", err)
	}
	logrus.Infof("Configuring public API body limit to %v bytes", publicLimit)
	limitingHandler := utils.APIBodyLimitingHandler(publicLimit)

	unauthed.Path("/").MatcherFunc(parse.MatchNotBrowser).Handler(managementAPI)
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
	if features.V3Public.Enabled() {
		unauthed.PathPrefix("/v3-public").Handler(v3PublicAPI)
	}
	unauthed.PathPrefix("/v1-public").Handler(v1PublicAPI)

	// Authenticated routes
	impersonatingAuth := requests.NewImpersonatingAuth(scaledContext.Wrangler, sar.NewSubjectAccessReview(clusterManager))
	saAuth := auth.ToMiddleware(requests.NewServiceAccountAuth(scaledContext, clustermanager.ToRESTConfig))
	accessControlHandler := rbac.NewAccessControlHandler()

	saauthed := mux.NewRouter()
	saauthed.UseEncodedPath()
	saauthed.PathPrefix("/k8s/clusters/{clusterID}").Handler(k8sProxy)
	saauthed.Use(mux.MiddlewareFunc(saAuth.Chain(impersonatingAuth.ImpersonationMiddleware)))
	saauthed.Use(mux.MiddlewareFunc(accessControlHandler))
	saauthed.Use(requests.NewAuthenticatedFilter)

	authed := mux.NewRouter()
	authed.UseEncodedPath()

	authed.Use(impersonatingAuth.ImpersonationMiddleware)
	authed.Use(mux.MiddlewareFunc(accessControlHandler))
	authed.Use(requests.NewAuthenticatedFilter)

	authed.Path("/meta/{resource:aks.+}").Handler(aks.NewAKSHandler(scaledContext))
	authed.Path("/meta/{resource:gke.+}").Handler(gke.NewGKEHandler(scaledContext))
	authed.Path("/meta/{resource:alibaba.+}").Handler(alibaba.NewAlibabaHandler(scaledContext))
	authed.Path("/meta/oci/{resource}").Handler(oci.NewOCIHandler(scaledContext))
	authed.Path("/meta/vsphere/{field}").Methods(http.MethodGet).Handler(vsphere.NewVsphereHandler(scaledContext))
	authed.Path("/v3/tokenreview").Methods(http.MethodPost).Handler(&webhook.TokenReviewer{})
	authed.Path(supportconfigs.Endpoint).Handler(&supportConfigGenerator)
	authed.PathPrefix("/meta/proxy").Handler(metaProxy)
	authed.PathPrefix("/v3/identit").Handler(tokenAPI)
	authed.PathPrefix("/v3/token").Handler(tokenAPI)
	authed.PathPrefix("/v3").Handler(managementAPI)
	authed.Methods(http.MethodPost).Path("/v1/logout").Handler(logout)

	// Metrics authenticated route
	metricsAuthed := mux.NewRouter()
	metricsAuthed.UseEncodedPath()
	tokenReviewAuth := auth.ToMiddleware(requests.NewTokenReviewAuth(scaledContext.K8sClient.AuthenticationV1()))
	metricsAuthed.Use(mux.MiddlewareFunc(tokenReviewAuth.Chain(impersonatingAuth.ImpersonationMiddleware)))
	metricsAuthed.Use(mux.MiddlewareFunc(accessControlHandler))
	metricsAuthed.Use(requests.NewAuthenticatedFilter)
	metricsAuthed.Use(metrics.NewMetricsHandler(scaledContext.K8sClient))
	metricsAuthed.Path("/metrics").Handler(promhttp.Handler())

	unauthed.NotFoundHandler = saauthed
	saauthed.NotFoundHandler = authed
	authed.NotFoundHandler = metricsAuthed

	return func(next http.Handler) http.Handler {
		metricsAuthed.NotFoundHandler = next
		return limitingHandler(unauthed)
	}, nil
}

// onlyGet will match only GET but will not return a 405 like route.Methods and instead just not match
func onlyGet(req *http.Request, m *mux.RouteMatch) bool {
	return req.Method == http.MethodGet
}
