package ui

import (
	"crypto/tls"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/pkg/settings"

	responsewriter "github.com/rancher/apiserver/pkg/middleware"
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

	ember      = newHandler(settings.UIIndex.Get, emberEnabled)
	vue        = newHandler(settings.DashboardIndex.Get, vueEnabled)
	emberIndex = ember.IndexFile()
	vueIndex   = vue.IndexFile()
)

func vueEnabled() bool {
	return true
}

func emberEnabled() bool {
	return settings.UIPreferred.Get() == "ember"
}

func newHandler(pathSetting func() string, enabled func() bool) *handler {
	return &handler{
		pathSetting: pathSetting,
		middleware: responsewriter.Chain{
			responsewriter.Gzip,
			responsewriter.DenyFrameOptions,
			responsewriter.CacheMiddleware("json", "js", "css"),
		}.Handler,
		indexMiddleware: responsewriter.Chain{
			responsewriter.Gzip,
			responsewriter.NoCache,
			responsewriter.DenyFrameOptions,
			responsewriter.ContentType,
		}.Handler,
	}
}

type handler struct {
	pathSetting     func() string
	enabledSetting  func() bool
	middleware      func(http.Handler) http.Handler
	indexMiddleware func(http.Handler) http.Handler
}

func (u *handler) path() (path string, isURL bool) {
	path = u.pathSetting()
	return path, strings.HasPrefix(path, "http")
}

func (u *handler) ServeAsset() http.Handler {
	return u.middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if path, isURL := u.path(); isURL {
			http.NotFound(rw, req)
		} else {
			http.FileServer(http.Dir(path)).ServeHTTP(rw, req)
		}
	}))
}

func (u *handler) IndexFile() http.Handler {
	return u.indexMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if path, isURL := u.path(); isURL {
			_ = serveIndex(rw, path)
		} else {
			http.ServeFile(rw, req, filepath.Join(path, "index.html"))
		}
	}))
}

func serveIndex(resp http.ResponseWriter, url string) error {
	r, err := insecureClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}
