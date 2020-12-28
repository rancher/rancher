package github

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type Proxy struct {
	proxy           *httputil.ReverseProxy
	secrets         v1.SecretCache
	secretNamespace string
	key             string
}

func NewProxy(secrets v1.SecretCache, githubURL, namespace, dataKey string) (http.Handler, error) {
	gURL, err := url.Parse(githubURL)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		proxy: &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				path := mux.Vars(req)["path"]
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
				if gURL.Path != "" {
					req.URL.Path = gURL.Path + req.URL.Path
				}
				req.URL.Path = path
				req.URL.Scheme = gURL.Scheme
				req.URL.Host = gURL.Host
				req.Host = gURL.Host
			},
		},
		secrets:         secrets,
		secretNamespace: namespace,
		key:             dataKey,
	}, nil
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		notFound(rw, "user not found")
		return
	}

	secretName := fmt.Sprintf("%s-secret", user.GetName())
	secret, err := p.secrets.Get(p.secretNamespace, secretName)
	if err != nil {
		notFound(rw, err.Error())
		return
	}

	token := string(secret.Data[p.key])
	if token == "" {
		notFound(rw, "token not found")
		return
	}

	req.Header.Del("Cookie")
	req.Header.Del("Authorization")
	req.SetBasicAuth(user.GetName(), token)
	p.proxy.ServeHTTP(rw, req)
}

func notFound(rw http.ResponseWriter, msg string) {
	rw.WriteHeader(http.StatusNotFound)
	if msg != "" {
		rw.Write([]byte(msg))
	}
}
