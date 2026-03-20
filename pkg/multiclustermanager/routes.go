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

	publicLimit, err := settings.APIBodyLimit.GetQuantityAsInt64(1024 * 1024)
	if err != nil {
		return nil, fmt.Errorf("parsing the public API body limit: %w", err)
	}
	logrus.Infof("Configuring public API body limit to %v bytes", publicLimit)
	limitingHandler := utils.APIBodyLimitingHandler(publicLimit)

	// Middleware composition helpers
	impersonatingAuth := requests.NewImpersonatingAuth(scaledContext.Wrangler, sar.NewSubjectAccessReview(clusterManager))
	saAuth := auth.ToMiddleware(requests.NewServiceAccountAuth(scaledContext, clustermanager.ToRESTConfig))
	accessControlHandler := rbac.NewAccessControlHandler()
	tokenReviewAuth := auth.ToMiddleware(requests.NewTokenReviewAuth(scaledContext.K8sClient.AuthenticationV1()))

	applyMiddleware := func(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			h = middlewares[i](h)
		}
		return h
	}

	saauthedWrap := func(h http.Handler) http.Handler {
		return applyMiddleware(h,
			saAuth.Chain(impersonatingAuth.ImpersonationMiddleware),
			accessControlHandler,
			requests.NewAuthenticatedFilter,
		)
	}

	authedWrap := func(h http.Handler) http.Handler {
		return applyMiddleware(h,
			impersonatingAuth.ImpersonationMiddleware,
			accessControlHandler,
			requests.NewAuthenticatedFilter,
		)
	}

	metricsAuthedWrap := func(h http.Handler) http.Handler {
		return applyMiddleware(h,
			tokenReviewAuth.Chain(impersonatingAuth.ImpersonationMiddleware),
			accessControlHandler,
			requests.NewAuthenticatedFilter,
			metrics.NewMetricsHandler(scaledContext.K8sClient),
		)
	}

	return func(next http.Handler) http.Handler {
		// Build the mux once when middleware is set up
		mux := http.NewServeMux()

		// Unauthenticated routes
		mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
			if parse.MatchNotBrowser(r) {
				managementAPI.ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
		mux.Handle("/v3/connect", connectHandler)
		mux.Handle("/v3/connect/register", connectHandler)
		mux.HandleFunc("/v3/import/{filename}", clusterImport.ClusterImportHandler)
		mux.HandleFunc("GET /v3/settings/cacerts", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.HandleFunc("GET /v3/settings/first-login", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.HandleFunc("GET /v3/settings/ui-banners", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.HandleFunc("GET /v3/settings/ui-issues", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.HandleFunc("GET /v3/settings/ui-pl", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.HandleFunc("GET /v3/settings/ui-brand", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.HandleFunc("GET /v3/settings/ui-default-landing", func(w http.ResponseWriter, r *http.Request) {
			managementAPI.ServeHTTP(w, r)
		})
		mux.Handle("/rancherversion", version.NewVersionHandler())
		// Channel server routes - handle /v1-k3s-release/* and /v1-rke2-release/*
		// Using specific prefixes since http.ServeMux doesn't support wildcards in path segments
		mux.Handle("/v1-k3s-release/", channelserver)
		mux.Handle("/v1-rke2-release/", channelserver)
		mux.Handle("/v1-saml/", saml.AuthHandler())
		if features.V3Public.Enabled() {
			mux.Handle("/v3-public/", v3PublicAPI)
		}
		mux.Handle("/v1-public/", v1PublicAPI)
		if features.SCIM.Enabled() {
			mux.Handle(scim.URLPrefix+"/", scim.NewHandler(scaledContext))
		}

		// Service account authenticated routes
		mux.Handle("/k8s/clusters/{clusterID}/", saauthedWrap(k8sProxy))
		mux.Handle("/k8s/proxy/{clusterID}/", saauthedWrap(k8sProxy))

		// Authenticated routes
		mux.HandleFunc("/meta/{resource}", func(w http.ResponseWriter, r *http.Request) {
			resource := r.PathValue("resource")
			var h http.Handler
			if strings.HasPrefix(resource, "aks") {
				h = aks.NewAKSHandler(scaledContext)
			} else if strings.HasPrefix(resource, "gke") {
				h = gke.NewGKEHandler(scaledContext)
			} else if strings.HasPrefix(resource, "alibaba") {
				h = alibaba.NewAlibabaHandler(scaledContext)
			} else {
				http.NotFound(w, r)
				return
			}
			authedWrap(h).ServeHTTP(w, r)
		})
		mux.Handle("/meta/oci/{resource}", authedWrap(oci.NewOCIHandler(scaledContext)))
		mux.HandleFunc("GET /meta/vsphere/{field}", func(w http.ResponseWriter, r *http.Request) {
			authedWrap(vsphere.NewVsphereHandler(scaledContext)).ServeHTTP(w, r)
		})
		mux.HandleFunc("POST /v3/tokenreview", func(w http.ResponseWriter, r *http.Request) {
			authedWrap(&webhook.TokenReviewer{}).ServeHTTP(w, r)
		})
		mux.Handle(supportconfigs.Endpoint, authedWrap(&supportConfigGenerator))
		mux.Handle("/meta/proxy/", authedWrap(metaProxy))
		mux.Handle("/v3/identit", authedWrap(tokenAPI))
		mux.Handle("/v3/token", authedWrap(tokenAPI))
		mux.Handle("/v3/", authedWrap(managementAPI))
		mux.HandleFunc("POST /v1/logout", func(w http.ResponseWriter, r *http.Request) {
			authedWrap(logout).ServeHTTP(w, r)
		})

		// Metrics authenticated route
		mux.Handle("/metrics", metricsAuthedWrap(promhttp.Handler()))

		// Final fallback for unmatched routes
		mux.Handle("/", next)

		return limitingHandler(mux)
	}, nil
}
