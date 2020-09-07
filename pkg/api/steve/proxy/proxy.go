package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	gmux "github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/proxy"
	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/endpoints/request"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

type Handler struct {
	authorizer    authorizer.Authorizer
	dialerFactory ClusterDialerFactory
	clusters      v3.ClusterCache
}

type ClusterDialerFactory interface {
	ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error)
}

func RewriteLocalCluster(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/k8s/clusters/local") {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/k8s/clusters/local")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		next.ServeHTTP(rw, req)
	})
}

func NewProxyMiddleware(sar v1.SubjectAccessReviewInterface,
	dialerFactory ClusterDialerFactory,
	clusters v3.ClusterCache,
	localSupport bool,
	localCluster http.Handler) (func(http.Handler) http.Handler, error) {
	cfg := authorizerfactory.DelegatingAuthorizerConfig{
		SubjectAccessReviewClient: sar,
		AllowCacheTTL:             time.Second * time.Duration(settings.AuthorizationCacheTTLSeconds.GetInt()),
		DenyCacheTTL:              time.Second * time.Duration(settings.AuthorizationDenyCacheTTLSeconds.GetInt()),
	}

	authorizer, err := cfg.New()
	if err != nil {
		return nil, err
	}

	proxyHandler := NewProxyHandler(authorizer, dialerFactory, clusters)

	mux := gmux.NewRouter()
	mux.UseEncodedPath()
	mux.Path("/v1/management.cattle.io.clusters/{clusterID}").Queries("link", "shell").HandlerFunc(routeToShellProxy(localSupport, localCluster, mux, proxyHandler))
	mux.Path("/v3/clusters/{clusterID}").Queries("shell", "true").HandlerFunc(routeToShellProxy(localSupport, localCluster, mux, proxyHandler))
	mux.Path("/{prefix:k8s/clusters/[^/]+}{suffix:/v1.*}").MatcherFunc(proxyHandler.MatchNonLegacy("/k8s/clusters/", true)).Handler(proxyHandler)
	mux.Path("/{prefix:k8s/clusters/[^/]+}{suffix:.*}").MatcherFunc(proxyHandler.MatchNonLegacy("/k8s/clusters/", false)).Handler(proxyHandler)

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			mux.NotFoundHandler = handler
			mux.ServeHTTP(rw, req)
		})
	}, nil
}

func routeToShellLink(rw http.ResponseWriter, req *http.Request, next http.Handler) {
	clusterID := gmux.Vars(req)["id"]
	if clusterID == "local" {
		req.URL.RawPath = "/v1/management.cattle.io.clusters/local"
	} else {
		req.URL.RawPath = fmt.Sprintf("/k8s/clusters/%s/v1/management.cattle.io.clusters/local", clusterID)
	}
	req.URL.Path = req.URL.RawPath
	req.URL.RawQuery = url.Values{
		"link": []string{"shell"},
	}.Encode()
	next.ServeHTTP(rw, req)
}

func routeToShellProxy(localSupport bool, localCluster http.Handler, mux *gmux.Router, proxyHandler *Handler) func(rw http.ResponseWriter, r *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		vars := gmux.Vars(r)
		cluster := vars["clusterID"]
		if cluster == "local" {
			if localSupport {
				q := r.URL.Query()
				q.Set("link", "shell")
				r.URL.RawQuery = q.Encode()
				r.URL.Path = "/v1/management.cattle.io.clusters/local"
				localCluster.ServeHTTP(rw, r)
			} else {
				mux.NotFoundHandler.ServeHTTP(rw, r)
			}
			return
		}
		vars["prefix"] = "k8s/clusters/" + cluster
		vars["suffix"] = "/v1/management.cattle.io.clusters/local"
		// Ensure shell link is set
		q := r.URL.Query()
		q.Set("link", "shell")
		r.URL.RawQuery = q.Encode()
		r.URL.Path = "/k8s/clusters/" + cluster + "/v1/management.cattle.io.clusters/local"
		proxyHandler.ServeHTTP(rw, r)
	}
}

func NewProxyHandler(authorizer authorizer.Authorizer,
	dialerFactory ClusterDialerFactory,
	clusters v3.ClusterCache) *Handler {
	return &Handler{
		authorizer:    authorizer,
		dialerFactory: dialerFactory,
		clusters:      clusters,
	}
}

func (h *Handler) MatchNonLegacy(prefix string, force bool) gmux.MatcherFunc {
	return func(req *http.Request, match *gmux.RouteMatch) bool {
		if !features.SteveProxy.Enabled() && !force {
			return false
		}

		clusterID := strings.TrimPrefix(req.URL.Path, prefix)
		clusterID = strings.SplitN(clusterID, "/", 2)[0]
		if match.Vars == nil {
			match.Vars = map[string]string{}
		}
		match.Vars["clusterID"] = clusterID

		cluster, err := h.clusters.Get(clusterID)
		if err != nil {
			return true
		}

		// No steve means we are upgrading a node that doesn't have the right proxy
		return cluster.Status.AgentFeatures[features.Steve.Name()]
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	prefix := "/" + gmux.Vars(req)["prefix"]
	clusterID := gmux.Vars(req)["clusterID"]

	if !h.canAccess(req.Context(), user, clusterID) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	handler, err := h.next(clusterID, prefix)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}

	handler.ServeHTTP(rw, req)
}

func (h *Handler) dialer(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	dialer := h.dialerFactory.ClusterDialer(host)
	return dialer(ctx, network, "127.0.0.1:6080")
}

func (h *Handler) next(clusterID, prefix string) (http.Handler, error) {
	cfg := &rest.Config{
		// this is bogus, the dialer will change it to 127.0.0.1:6080, but the clusterID is used to lookup the tunnel
		// connect
		Host:      "http://" + clusterID,
		UserAgent: rest.DefaultKubernetesUserAgent() + " cluster " + clusterID,
		// Ensure this function pointer does not change per invocation so that we don't
		// blow out the cache.
		Dial: h.dialer,
	}

	next := proxy.ImpersonatingHandler(prefix, cfg)
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req.Header.Set("X-API-URL-Prefix", prefix)
		next.ServeHTTP(rw, req)
	}), nil
}

func (h *Handler) canAccess(ctx context.Context, user user.Info, clusterID string) bool {
	extra := map[string]authzv1.ExtraValue{}
	for k, v := range user.GetExtra() {
		extra[k] = v
	}

	resp, _, err := h.authorizer.Authorize(ctx, authorizer.AttributesRecord{
		ResourceRequest: true,
		User:            user,
		Verb:            "get",
		APIGroup:        managementv3.GroupName,
		APIVersion:      managementv3.Version,
		Resource:        "clusters",
		Name:            clusterID,
	})

	return err == nil && resp == authorizer.DecisionAllow
}
