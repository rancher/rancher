package dashboard

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/parse"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/server/responsewriter"
	"github.com/sirupsen/logrus"
)

var (
	insecureClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

func content() http.Handler {
	return http.FileServer(http.Dir(uipath()))
}

func uipath() string {
	return filepath.Join(settings.UIPath.Get(), "dashboard")
}

func Route(next http.Handler) http.Handler {
	uiContent := responsewriter.NewMiddlewareChain(responsewriter.Gzip,
		responsewriter.DenyFrameOptions,
		responsewriter.CacheMiddleware("json", "js", "css")).Handler(content())

	root := mux.NewRouter()
	root.Path("/dashboard").HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add("Location", "/dashboard/")
		rw.WriteHeader(http.StatusFound)
	})
	root.PathPrefix("/dashboard/assets").Handler(uiContent)
	root.PathPrefix("/dashboard/translations").Handler(uiContent)
	root.PathPrefix("/dashboard/engines-dist").Handler(uiContent)
	root.Handle("/dashboard/asset-manifest.json", uiContent)
	root.Handle("/dashboard/index.html", uiContent)
	root.PathPrefix("/dashboard/").Handler(wrapUI(next))
	root.NotFoundHandler = next

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/k8s/clusters/local") {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/k8s/clusters/local")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		root.ServeHTTP(rw, req)
	})
}

func wrapUI(next http.Handler) http.Handler {
	_, err := os.Stat(indexHTML())
	local := err == nil
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			if local && settings.UIIndex.Get() == "local" {
				http.ServeFile(resp, req, indexHTML())
			} else {
				ui(resp, req)
			}
		} else {
			next.ServeHTTP(resp, req)
		}
	})
}

func indexHTML() string {
	return filepath.Join(uipath(), "index.html")
}

func ui(resp http.ResponseWriter, req *http.Request) {
	if err := serveIndex(resp, req); err != nil {
		logrus.Errorf("failed to serve UI: %v", err)
		resp.WriteHeader(500)
	}
}

func serveIndex(resp http.ResponseWriter, req *http.Request) error {
	r, err := insecureClient.Get(settings.DashboardIndex.Get())
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}
