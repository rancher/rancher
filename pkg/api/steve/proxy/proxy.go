package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/disallow"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/remotedialer"
	"github.com/rancher/steve/pkg/auth"
	"github.com/rancher/steve/pkg/proxy"
	"github.com/sirupsen/logrus"
	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/endpoints/request"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

type Handler struct {
	authorizer         authorizer.Authorizer
	dialerFactory      ClusterDialerFactory
	requestInfoFactory request.RequestInfoFactory
}

type ClusterDialerFactory func(clusterID string) remotedialer.Dialer

func RewriteLocalCluster(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/k8s/clusters/local") {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/k8s/clusters/local")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		} else if strings.HasPrefix(req.URL.Path, "/k8s/proxy/local") {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/k8s/proxy/local")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		next.ServeHTTP(rw, req)
	})
}

func NewProxyMiddleware(sar v1.AuthorizationV1Interface,
	dialerFactory ClusterDialerFactory,
	clusters v3.ClusterCache,
	localSupport bool,
	localCluster http.Handler) (func(http.Handler) http.Handler, error) {
	cfg := authorizerfactory.DelegatingAuthorizerConfig{
		SubjectAccessReviewClient: sar,
		AllowCacheTTL:             time.Second * time.Duration(settings.AuthorizationCacheTTLSeconds.GetInt()),
		DenyCacheTTL:              time.Second * time.Duration(settings.AuthorizationDenyCacheTTLSeconds.GetInt()),
		WebhookRetryBackoff:       &auth.WebhookBackoff,
	}

	authorizer, err := cfg.New()
	if err != nil {
		return nil, err
	}

	proxyHandler := NewProxyHandler(authorizer, dialerFactory, clusters)

	return func(handler http.Handler) http.Handler {
		mux := http.NewServeMux()

		// Handle /api paths with management CRD matching
		mux.HandleFunc("/api/", func(rw http.ResponseWriter, req *http.Request) {
			if proxyHandler.matchManagementCRDs(req) {
				proxyHandler.authLocalCluster(handler, rw, req)
				return
			}
			handler.ServeHTTP(rw, req)
		})

		// Handle shell/apply routes with query parameters
		mux.HandleFunc("/v1/management.cattle.io.clusters/{clusterID}", func(rw http.ResponseWriter, req *http.Request) {
			link := req.URL.Query().Get("link")
			action := req.URL.Query().Get("action")

			if link == "shell" {
				routeToShellProxy("link", "shell", localSupport, localCluster, handler, proxyHandler)(rw, req)
				return
			}
			if action == "apply" {
				routeToShellProxy("action", "apply", localSupport, localCluster, handler, proxyHandler)(rw, req)
				return
			}
			handler.ServeHTTP(rw, req)
		})

		mux.HandleFunc("/v3/clusters/{clusterID}", func(rw http.ResponseWriter, req *http.Request) {
			if req.URL.Query().Get("shell") == "true" {
				routeToShellProxy("link", "shell", localSupport, localCluster, handler, proxyHandler)(rw, req)
				return
			}
			handler.ServeHTTP(rw, req)
		})

		// Handle k8s cluster proxy paths
		mux.HandleFunc("/k8s/clusters/{clusterID}/", func(rw http.ResponseWriter, req *http.Request) {
			clusterID := req.PathValue("clusterID")
			// Extract the suffix after /k8s/clusters/{clusterID}/
			prefix := "/k8s/clusters/" + clusterID
			suffix := strings.TrimPrefix(req.URL.Path, prefix)

			// Only handle if suffix starts with /v1
			if strings.HasPrefix(suffix, "/v1") {
				proxyHandler.ServeHTTPWithCluster(rw, req, clusterID, prefix)
				return
			}
			handler.ServeHTTP(rw, req)
		})

		mux.Handle("/", handler)
		return mux
	}, nil
}

func routeToShellProxy(key, value string, localSupport bool, localCluster http.Handler, notFoundHandler http.Handler, proxyHandler *Handler) func(rw http.ResponseWriter, r *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		cluster := r.PathValue("clusterID")
		if cluster == "local" {
			if localSupport {
				authed := proxyHandler.userCanAccessCluster(r, cluster)
				if !authed {
					rw.WriteHeader(http.StatusUnauthorized)
					return
				}
				q := r.URL.Query()
				q.Set(key, value)
				r.URL.RawQuery = q.Encode()
				r.URL.Path = "/v1/management.cattle.io.clusters/local"
				localCluster.ServeHTTP(rw, r)
			} else {
				notFoundHandler.ServeHTTP(rw, r)
			}
			return
		}
		q := r.URL.Query()
		q.Set(key, value)
		r.URL.RawQuery = q.Encode()
		r.URL.Path = "/k8s/clusters/" + cluster + "/v1/management.cattle.io.clusters/local"
		proxyHandler.ServeHTTPWithCluster(rw, r, cluster, "/k8s/clusters/"+cluster)
	}
}

func NewProxyHandler(authorizer authorizer.Authorizer,
	dialerFactory ClusterDialerFactory,
	clusters v3.ClusterCache) *Handler {
	return &Handler{
		authorizer:         authorizer,
		dialerFactory:      dialerFactory,
		requestInfoFactory: request.RequestInfoFactory{APIPrefixes: sets.NewString("apis", "api"), GrouplessAPIPrefixes: sets.NewString("api")},
	}
}

func (h *Handler) authLocalCluster(notFoundHandler http.Handler, rw http.ResponseWriter, req *http.Request) {
	authed := h.userCanAccessCluster(req, "local")
	if !authed {
		rw.WriteHeader(http.StatusForbidden)
		return
	}
	notFoundHandler.ServeHTTP(rw, req)
}

// matchManagementCRDs matches paths that are for management CRDs that are not in the allow-list of specific management resources.
// To decide what to match, it tries to extract request information from the URL path and examine the group and resource.
func (h *Handler) matchManagementCRDs(req *http.Request) bool {
	info, err := h.requestInfoFactory.NewRequestInfo(req)
	if err != nil {
		// This isn't a K8s request, don't match it.
		return false
	}
	return info.APIGroup == managementv3.GroupName && info.Resource != "" && !disallow.AllowAll[info.Resource]
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	clusterID := req.PathValue("clusterID")
	h.ServeHTTPWithCluster(rw, req, clusterID, "/k8s/clusters/"+clusterID)
}

func (h *Handler) ServeHTTPWithCluster(rw http.ResponseWriter, req *http.Request, clusterID, prefix string) {
	authed := h.userCanAccessCluster(req, clusterID)
	if !authed {
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

func (h *Handler) userCanAccessCluster(req *http.Request, clusterID string) bool {
	requestUser, ok := request.UserFrom(req.Context())
	if ok {
		return h.canAccess(req.Context(), requestUser, clusterID)
	}
	return false
}

func (h *Handler) dialer(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	dialer := h.dialerFactory("stv-cluster-" + host)
	var conn net.Conn
	for i := 0; i < 15; i++ {
		conn, err = dialer(ctx, network, "127.0.0.1:6080")
		if err != nil && strings.Contains(err.Error(), "failed to find Session for client") {
			if i < 14 {
				logrus.Tracef("steve.proxy.dialer: lost connection, retrying")
				time.Sleep(time.Second)
			} else {
				logrus.Tracef("steve.proxy.dialer: lost connection, failed to reconnect after 15 attempts")
			}
		} else {
			break
		}
	}
	if err != nil {
		return conn, fmt.Errorf("lost connection to cluster: %w", err)
	}
	return conn, nil
}

func (h *Handler) next(clusterID, prefix string) (http.Handler, error) {
	ht := http.DefaultTransport.(*http.Transport).Clone()
	ht.Proxy = nil
	ht.DialContext = h.dialer
	cfg := &rest.Config{
		// this is bogus, the dialer will change it to 127.0.0.1:6080, but the clusterID is used to lookup the tunnel
		// connect
		Host:      "http://" + clusterID,
		UserAgent: rest.DefaultKubernetesUserAgent() + " cluster " + clusterID,
		Transport: ht,
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
