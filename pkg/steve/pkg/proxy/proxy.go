package proxy

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/remotedialer"
	"github.com/rancher/steve/pkg/proxy"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/rest"
)

type handler struct {
	authorizer   authorizer.Authorizer
	tunnelServer *remotedialer.Server
}

func NewProxyHandler(authorizer authorizer.Authorizer,
	tunnelServer *remotedialer.Server) http.Handler {
	return &handler{
		authorizer:   authorizer,
		tunnelServer: tunnelServer,
	}
}

func (h *handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	prefix, ok := mux.Vars(req)["prefix"]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if !strings.HasPrefix(prefix, "/") {
		// may not include first slash and should
		prefix = "/" + prefix
	}

	parts := strings.Split(prefix, "/")
	clusterID := parts[len(parts)-1]

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

func (h *handler) dialer(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	dialer := h.tunnelServer.Dialer(host, 15*time.Second)
	return dialer(network, "127.0.0.1:6443")
}

func (h *handler) next(clusterID, prefix string) (http.Handler, error) {
	cfg := &rest.Config{
		// this is bogus, the dialer will change it to 127.0.0.1:6443
		Host:      "https://" + clusterID,
		UserAgent: rest.DefaultKubernetesUserAgent() + " cluster " + clusterID,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
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

func (h *handler) canAccess(ctx context.Context, user user.Info, clusterID string) bool {
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
