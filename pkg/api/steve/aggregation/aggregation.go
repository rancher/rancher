package aggregation

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/remotedialer"
	"github.com/rancher/steve/pkg/proxy"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
)

var (
	clusterPrefixRegexp = regexp.MustCompile(`^/k8s/(clusters|proxy)/[^/]+`)
)

type aggregationHandler struct {
	sync.Mutex

	apiServiceCache mgmtcontrollers.APIServiceCache
	routes          []routeEntry
	remote          *remotedialer.Server
}

type routeEntry struct {
	path   string
	prefix string
	uuid   string
}

func NewMiddleware(ctx context.Context, apiServices mgmtcontrollers.APIServiceController, remotedialer *remotedialer.Server) func(http.Handler) http.Handler {
	handler := &aggregationHandler{
		apiServiceCache: apiServices.Cache(),
		remote:          remotedialer,
	}
	relatedresource.WatchClusterScoped(ctx, "aggregation-router", relatedresource.TriggerAllKey,
		apiServices, apiServices)
	apiServices.OnChange(ctx, "apiservice-router", handler.OnChange)
	return handler.Middleware
}

func (h *aggregationHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		h.next(next).ServeHTTP(rw, req)
	})
}

func (h *aggregationHandler) next(notFound http.Handler) http.Handler {
	h.Lock()
	defer h.Unlock()
	if len(h.routes) == 0 {
		return notFound
	}

	// Rebuild mux with current routes and notFound fallback
	mux := http.NewServeMux()
	for _, entry := range h.routes {
		if entry.prefix != "" {
			// For prefix matching, ensure trailing slash
			prefix := entry.prefix
			if !strings.HasSuffix(prefix, "/") {
				prefix = prefix + "/"
			}
			mux.Handle(prefix, h.makeHandler(entry.uuid))
		}
		if entry.path != "" {
			mux.Handle(entry.path, h.makeHandler(entry.uuid))
		}
	}
	mux.Handle("/", notFound)
	return mux
}

func (h *aggregationHandler) setEntries(routes []routeEntry) {
	h.Lock()
	defer h.Unlock()
	h.routes = routes
}

func keyFromUUID(uuid string) string {
	return "stv-" + uuid
}

func (h *aggregationHandler) makeHandler(uuid string) http.Handler {
	key := keyFromUUID(uuid)
	cfg := &rest.Config{
		Host:      "http://" + key,
		UserAgent: rest.DefaultKubernetesUserAgent() + " " + key,
		Transport: &http.Transport{
			DialContext: h.remote.Dialer(key),
		},
	}

	next := proxy.ImpersonatingHandler("", cfg)
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		for i := 0; i < 15; i++ {
			if !h.remote.HasSession(key) {
				time.Sleep(time.Second)
			}
		}
		if !h.remote.HasSession(key) {
			http.Error(rw, "Handler disconnected", http.StatusServiceUnavailable)
			return
		}

		if prefix := clusterPrefixRegexp.FindString(req.URL.Path); prefix != "" {
			req.Header.Set("X-API-URL-Prefix", prefix)
		}

		next.ServeHTTP(rw, req)
	})
}

func (h *aggregationHandler) OnChange(key string, obj *v3.APIService) (*v3.APIService, error) {
	if key != relatedresource.AllKey {
		return obj, nil
	}

	apiServices, err := h.apiServiceCache.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.Slice(apiServices, func(i, j int) bool {
		return apiServices[i].Name < apiServices[j].Name
	})

	var entries []routeEntry
	for _, apiService := range apiServices {
		for _, prefix := range apiService.Spec.PathPrefixes {
			entries = append(entries, routeEntry{
				prefix: prefix,
				uuid:   string(apiService.UID),
			})
		}
		for _, path := range apiService.Spec.Paths {
			entries = append(entries, routeEntry{
				path: path,
				uuid: string(apiService.UID),
			})
		}
	}

	h.setEntries(entries)
	return obj, nil
}
