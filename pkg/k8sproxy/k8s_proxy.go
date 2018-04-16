package k8sproxy

import (
	"net/http"

	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/rancher/pkg/clusterrouter/proxy"
	rdialer "github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/k8slookup"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
)

func New(scaledContext *config.ScaledContext, dialer dialer.Factory) http.Handler {
	return clusterrouter.New(scaledContext.LocalConfig, k8slookup.New(scaledContext, true), dialer,
		scaledContext.Management.Clusters("").Controller().Lister())
}

func NewLocalProxy(scaledContext *config.ScaledContext, dialer dialer.Factory, next http.Handler) (http.Handler, error) {
	passThrough, err := proxy.NewSimpleProxy(scaledContext.LocalConfig.Host, scaledContext.LocalConfig.CAData)
	if err != nil {
		return nil, err
	}

	router := clusterrouter.New(scaledContext.LocalConfig, k8slookup.New(scaledContext, false), dialer,
		scaledContext.Management.Clusters("").Controller().Lister())

	lp := &localProxy{
		passThrough: passThrough,
		next:        next,
		router:      router,
	}

	if rd, ok := dialer.(*rdialer.Factory); ok {
		lp.auth = rd.TunnelAuthorizer
	}

	return lp, nil
}

type localProxy struct {
	passThrough http.Handler
	next        http.Handler
	auth        *tunnelserver.Authorizer
	router      http.Handler
}

func (l *localProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if l.pass(rw, req) {
		return
	}

	if !l.proxy(rw, req) {
		l.next.ServeHTTP(rw, req)
	}
}

func (l *localProxy) pass(rw http.ResponseWriter, req *http.Request) bool {
	if req.Header.Get("X-API-K8s-Node-Client") == "true" {
		l.passThrough.ServeHTTP(rw, req)
		return true
	}
	return false
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
