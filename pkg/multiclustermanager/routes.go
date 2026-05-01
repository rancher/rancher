package multiclustermanager

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	"github.com/rancher/rancher/pkg/auth/providers/scim"
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
		clusterImport  = clusterregistrationtokens.ClusterImport{Clusters: scaledContext.Management.Clusters(""), SecretLister: scaledContext.Core.Secrets("").Controller().Lister()}
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
	unauthed := http.NewServeMux()

	publicLimit, err := settings.APIBodyLimit.GetQuantityAsInt64(1024 * 1024)
	if err != nil {
		return nil, fmt.Errorf("parsing the public API body limit: %w", err)
	}
	logrus.Infof("Configuring public API body limit to %v bytes", publicLimit)
	limitingHandler := utils.APIBodyLimitingHandler(publicLimit)

	impersonatingAuth := requests.NewImpersonatingAuth(scaledContext.Wrangler, sar.NewSubjectAccessReview(clusterManager))
	saAuth := auth.ToMiddleware(requests.NewServiceAccountAuth(scaledContext, clustermanager.ToRESTConfig))
	accessControlHandler := rbac.NewAccessControlHandler()

	// Setup middlewares for the service account authenticated routes.
	saAuthedMW := func(h http.Handler) http.Handler {
		h = requests.NewAuthenticatedFilter(h)
		h = accessControlHandler(h)
		h = saAuth.Chain(impersonatingAuth.ImpersonationMiddleware)(h)
		return h
	}
	saAuthed := http.NewServeMux()
	saAuthed.Handle("/k8s/clusters/{clusterID}/", saAuthedMW(k8sProxy))
	saAuthed.Handle("/k8s/proxy/{clusterID}/", saAuthedMW(k8sProxy))

	unauthed.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && parse.MatchNotBrowser(r) {
			managementAPI.ServeHTTP(w, r)
			return
		}
		saAuthed.ServeHTTP(w, r)
	}))
	unauthed.Handle("/v3/connect", connectHandler)
	unauthed.Handle("/v3/connect/register", connectHandler)
	unauthed.HandleFunc("/v3/import/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := r.PathValue("filename")
		// Match pattern: {token}_{clusterId}.yaml
		if strings.Contains(filename, "_") && strings.HasSuffix(filename, ".yaml") {
			clusterImport.ClusterImportHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
	unauthed.Handle("GET /v3/settings/cacerts", managementAPI)
	unauthed.Handle("GET /v3/settings/first-login", managementAPI)
	unauthed.Handle("GET /v3/settings/ui-banners", managementAPI)
	unauthed.Handle("GET /v3/settings/ui-issues", managementAPI)
	unauthed.Handle("GET /v3/settings/ui-pl", managementAPI)
	unauthed.Handle("GET /v3/settings/ui-brand", managementAPI)
	unauthed.Handle("GET /v3/settings/ui-default-landing", managementAPI)
	unauthed.Handle("/rancherversion", version.NewVersionHandler())
	unauthed.Handle("/v1-k3s-release/", channelserver)
	unauthed.Handle("/v1-rke2-release/", channelserver)
	unauthed.Handle("/v1-saml/", saml.AuthHandler())
	if features.V3Public.Enabled() {
		unauthed.Handle("/v3-public/", v3PublicAPI)
	}
	unauthed.Handle("/v1-public/", v1PublicAPI)
	if features.SCIM.Enabled() {
		unauthed.Handle(fmt.Sprint(scim.URLPrefix, "/"), scim.NewHandler(scaledContext))
	}

	// Setup middlewares for the metrics route.
	tokenReviewAuth := auth.ToMiddleware(requests.NewTokenReviewAuth(scaledContext.K8sClient.AuthenticationV1()))
	metricsMW := func(h http.Handler) http.Handler {
		h = metrics.NewMetricsHandler(scaledContext.K8sClient)(h)
		h = requests.NewAuthenticatedFilter(h)
		h = accessControlHandler(h)
		h = tokenReviewAuth.Chain(impersonatingAuth.ImpersonationMiddleware)(h)
		return h
	}
	metricsAuthed := http.NewServeMux()
	metricsAuthed.Handle("/metrics", metricsMW(promhttp.Handler()))

	// Setup middlewares for authenticated routes.
	authedMW := func(h http.Handler) http.Handler {
		h = requests.NewAuthenticatedFilter(h)
		h = accessControlHandler(h)
		h = impersonatingAuth.ImpersonationMiddleware(h)
		return h
	}

	authed := http.NewServeMux()
	authed.Handle("/meta/{resource}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := r.PathValue("resource")
		var h http.Handler
		if strings.HasPrefix(resource, "aks") {
			h = authedMW(aks.NewAKSHandler(scaledContext))
		} else if strings.HasPrefix(resource, "gke") {
			h = authedMW(gke.NewGKEHandler(scaledContext))
		} else if strings.HasPrefix(resource, "alibaba") {
			h = authedMW(alibaba.NewAlibabaHandler(scaledContext))
		} else {
			metricsAuthed.ServeHTTP(w, r)
			return
		}
		h.ServeHTTP(w, r)
	}))
	authed.Handle("/meta/oci/{resource}", authedMW(oci.NewOCIHandler(scaledContext)))
	authed.Handle("GET /meta/vsphere/{field}", authedMW(vsphere.NewVsphereHandler(scaledContext)))
	authed.Handle("POST /v3/tokenreview", authedMW(&webhook.TokenReviewer{}))
	authed.Handle(supportconfigs.Endpoint, authedMW(&supportConfigGenerator))
	authed.Handle("/meta/proxy/", authedMW(metaProxy))
	authed.Handle("/v3/", authedMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v3/identit") || strings.HasPrefix(r.URL.Path, "/v3/token") {
			tokenAPI.ServeHTTP(w, r)
		} else {
			managementAPI.ServeHTTP(w, r)
		}
	})))
	authed.Handle("POST /v1/logout", authedMW(logout))
	saAuthed.Handle("/", authed)
	authed.Handle("/", metricsAuthed)

	var nextHandler http.Handler
	metricsAuthed.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nextHandler != nil {
			nextHandler.ServeHTTP(w, r)
		}
	}))

	return func(next http.Handler) http.Handler {
		nextHandler = next
		return limitingHandler(unauthed)
	}, nil
}
