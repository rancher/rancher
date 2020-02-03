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
	v1 "github.com/rancher/types/apis/management.cattle.io/v3"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	typedauthzv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

type handler struct {
	sarGetter    typedauthzv1.SubjectAccessReviewsGetter
	tunnelServer *remotedialer.Server
}

func NewProxyHandler(sarGetter typedauthzv1.SubjectAccessReviewsGetter,
	tunnelServer *remotedialer.Server) http.Handler {
	return &handler{
		sarGetter:    sarGetter,
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

	if !h.canAccess(user, clusterID) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	dialer := h.tunnelServer.Dialer(clusterID, 15*time.Second)
	handler, err := h.next(clusterID, prefix, dialer)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}

	handler.ServeHTTP(rw, req)
}

func (h *handler) next(clusterID, prefix string, dialer remotedialer.Dialer) (http.Handler, error) {
	cfg := &rest.Config{
		Host:      "https://127.0.0.1:8443",
		UserAgent: rest.DefaultKubernetesUserAgent() + " cluster " + clusterID,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer(network, address)
		},
	}

	next, err := proxy.Handler(prefix, cfg)
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req.Header.Set("X-API-URL-Prefix", prefix)
		next.ServeHTTP(rw, req)
	}), nil
}

func (h *handler) canAccess(user user.Info, clusterID string) bool {
	extra := map[string]authzv1.ExtraValue{}
	for k, v := range user.GetExtra() {
		extra[k] = v
	}

	sar, err := h.sarGetter.SubjectAccessReviews().Create(&authzv1.SubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: authzv1.SubjectAccessReviewSpec{
			UID:    user.GetUID(),
			User:   user.GetName(),
			Groups: user.GetGroups(),
			Extra:  extra,
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:     "get",
				Group:    v1.GroupName,
				Version:  v1.Version,
				Resource: "clusters",
				Name:     clusterID,
			},
		},
		Status: authzv1.SubjectAccessReviewStatus{},
	})

	return err == nil && sar.Status.Allowed
}
