package dashboard

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/middleware"
	"github.com/rancher/apiserver/pkg/parse"
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

func content(uiSetting func() string) http.Handler {
	return http.FileServer(http.Dir(uiSetting()))
}

func Route(next http.Handler, uiSetting func() string) http.Handler {
	uiContent := middleware.NewMiddlewareChain(middleware.Gzip,
		middleware.DenyFrameOptions,
		middleware.CacheMiddleware("json", "js", "css")).Handler(content(uiSetting))

	root := mux.NewRouter()
	root.UseEncodedPath()
	root.Path("/").HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, "/dashboard/", http.StatusFound)
	})
	root.Path("/dashboard").HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, "/dashboard/", http.StatusFound)
	})
	root.PathPrefix("/dashboard/assets").Handler(uiContent)
	root.PathPrefix("/dashboard/translations").Handler(uiContent)
	root.PathPrefix("/dashboard/engines-dist").Handler(uiContent)
	root.Handle("/dashboard/asset-manifest.json", uiContent)
	root.Handle("/dashboard/index.html", uiContent)
	root.PathPrefix("/dashboard/").Handler(wrapUI(next, uiSetting))
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

func wrapUI(next http.Handler, uiGetter func() string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			path := uiGetter()
			if strings.HasPrefix(path, "http") {
				ui(resp, req, path)
			} else {
				http.ServeFile(resp, req, path)
			}
		} else {
			next.ServeHTTP(resp, req)
		}
	})
}

func ui(resp http.ResponseWriter, req *http.Request, url string) {
	if err := serveIndex(resp, req, url); err != nil {
		logrus.Errorf("failed to serve UI: %v", err)
		resp.WriteHeader(500)
	}
}

func serveIndex(resp http.ResponseWriter, req *http.Request, url string) error {
	r, err := insecureClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}
