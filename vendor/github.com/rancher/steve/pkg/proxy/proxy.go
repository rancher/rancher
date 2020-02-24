package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// Mostly copied from "kubectl proxy" code
func HandlerFromConfig(prefix, kubeConfig string) (http.Handler, error) {
	loader := kubeconfig.GetInteractiveClientConfig(kubeConfig)
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, err
	}

	return Handler(prefix, cfg)
}

func ImpersonatingHandler(prefix string, cfg *rest.Config) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		impersonate(rw, req, prefix, cfg)
	})
}

func impersonate(rw http.ResponseWriter, req *http.Request, prefix string, cfg *rest.Config) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	cfg = rest.CopyConfig(cfg)
	cfg.Impersonate.UserName = user.GetName()
	cfg.Impersonate.Groups = user.GetGroups()
	cfg.Impersonate.Extra = user.GetExtra()

	handler, err := Handler(prefix, cfg)
	if err != nil {
		logrus.Errorf("failed to impersonate %v for proxy: %v", user, err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	handler.ServeHTTP(rw, req)
}

// Mostly copied from "kubectl proxy" code
func Handler(prefix string, cfg *rest.Config) (http.Handler, error) {
	host := cfg.Host
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	target, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, err
	}
	upgradeTransport, err := makeUpgradeTransport(cfg, transport)
	if err != nil {
		return nil, err
	}

	proxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, er)
	proxy.UpgradeTransport = upgradeTransport
	proxy.UseRequestLocation = true

	handler := http.Handler(proxy)

	if len(target.Path) > 1 {
		handler = prependPath(target.Path[:len(target.Path)-1], handler)
	}

	if len(prefix) > 2 {
		handler = stripLeaveSlash(prefix, handler)
	}

	return authHeaders(handler), nil
}

func authHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req.Header.Del("Authorization")
		handler.ServeHTTP(rw, req)
	})
}

func SetHost(host string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.Host = host
		h.ServeHTTP(w, req)
	})
}

func prependPath(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if len(req.URL.Path) > 1 {
			req.URL.Path = prefix + req.URL.Path
		} else {
			req.URL.Path = prefix
		}
		h.ServeHTTP(w, req)
	})
}

// like http.StripPrefix, but always leaves an initial slash. (so that our
// regexps will work.)
func stripLeaveSlash(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, prefix)
		if len(p) > 0 && p[:1] != "/" {
			p = "/" + p
		}
		req.URL.Path = p
		fmt.Println(req.Method, " ", req.URL.String())
		h.ServeHTTP(w, req)
	})
}

func makeUpgradeTransport(config *rest.Config, rt http.RoundTripper) (proxy.UpgradeRequestRoundTripper, error) {
	transportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}

	upgrader, err := transport.HTTPWrappersForConfig(transportConfig, proxy.MirrorRequest)
	if err != nil {
		return nil, err
	}

	return proxy.NewUpgradeRequestRoundTripper(rt, upgrader), nil
}
