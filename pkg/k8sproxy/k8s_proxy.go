package k8sproxy

import (
	"net/http"

	"github.com/rancher/rancher/pkg/clusterrouter"
	rdialer "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/k8slookup"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
)

func New(scaledContext *config.ScaledContext, dialer dialer.Factory) http.Handler {
	return clusterrouter.New(scaledContext.LocalConfig, k8slookup.New(scaledContext, true), dialer)
}

func NewLocalProxy(scaledContext *config.ScaledContext, dialer dialer.Factory, next http.Handler) http.Handler {
	lp := &localProxy{
		next:   next,
		router: clusterrouter.New(scaledContext.LocalConfig, k8slookup.New(scaledContext, false), dialer),
	}

	if rd, ok := dialer.(*rdialer.Factory); ok {
		lp.auth = rd.TunnelAuthorizer
	}

	return lp
}

type localProxy struct {
	next   http.Handler
	auth   *tunnelserver.Authorizer
	router http.Handler
}

func (l *localProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !l.proxy(rw, req) {
		l.next.ServeHTTP(rw, req)
	}
}

func (l *localProxy) proxy(rw http.ResponseWriter, req *http.Request) bool {
	if l.auth == nil {
		return false
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		return false
	}

	user, groups, cluster, ok := l.auth.AuthorizeLocalNode(username, password)
	if !ok {
		return false
	}

	req.Header.Set("X-API-Cluster-Id", cluster.Name)
	req.Header.Set("Impersonate-User", user)
	for _, group := range groups {
		req.Header.Set("Impersonate-Group", group)
	}

	l.router.ServeHTTP(rw, req)
	return true
}
