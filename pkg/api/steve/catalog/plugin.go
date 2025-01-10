package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	neturl "net/url"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/controllers/dashboard/plugin"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type denyFunc func(host string) bool

func RegisterUIPluginHandlers(router *mux.Router) {
	router.HandleFunc("/v1/uiplugins", indexHandler)
	router.HandleFunc("/v1/uiplugins/{name}/{version}/{rest:.*}", pluginHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	var in *plugin.SafeIndex
	if isAuthenticated(r) {
		in = &plugin.Index
	} else {
		in = &plugin.AnonymousIndex
	}
	index, err := json.Marshal(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logrus.Error(err)
	}
	w.Write(index)
}

func pluginHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logrus.Debugf("http request vars %s", vars)
	authed := isAuthenticated(r)
	entry, ok := plugin.Index.Entries[vars["name"]]
	// Checks if the requested plugin exists and if the user has authorization to see it
	if (!ok || entry.Version != vars["version"]) || (!authed && !entry.NoAuth) {
		msg := fmt.Sprintf("plugin [name: %s version: %s] does not exist in index", vars["name"], vars["version"])
		http.Error(w, msg, http.StatusNotFound)
		logrus.Debug(msg)
		return
	}
	if entry.NoCache || entry.CacheState == plugin.Pending {
		logrus.Debugf("[noCache: %v] proxying request to [endpoint: %v]\n", entry.NoCache, entry.Endpoint)
		proxyRequest(entry.Endpoint, vars["rest"], w, r, denylist)
	} else {
		logrus.Debugf("[noCache: %v] serving plugin files from filesystem cache\n", entry.NoCache)
		r.URL.Path = fmt.Sprintf("/%s/%s/%s", vars["name"], vars["version"], vars["rest"])
		http.FileServer(http.Dir(plugin.FSCacheRootDir)).ServeHTTP(w, r)
	}
}

func proxyRequest(target, path string, w http.ResponseWriter, r *http.Request, denyListFunc denyFunc) {
	url, err := neturl.Parse(target)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse url [%s]", target), http.StatusInternalServerError)
		return
	}
	if denyListFunc(url.Hostname()) {
		http.Error(w, fmt.Sprintf("url [%s] is forbidden", target), http.StatusForbidden)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode == http.StatusOK {
			if contentType := mime.TypeByExtension(filepath.Ext(r.URL.Path)); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			} else {
				body, _ := io.ReadAll(response.Body)
				response.Body = io.NopCloser(bytes.NewBuffer(body))
				w.Header().Set("Content-Type", http.DetectContentType(body))
			}

		}
		return nil
	}
	r.URL.Host = url.Host
	r.URL.Scheme = url.Scheme
	r.URL.Path = path
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = url.Host
	proxy.ServeHTTP(w, r)
}

func denylist(host string) bool {
	denied := map[string]struct{}{
		"localhost":       {},
		"127.0.0.1":       {},
		"0.0.0.0":         {},
		"169.254.169.254": {},
		"::1":             {},
		"::":              {},
		"":                {},
	}
	_, isDenied := denied[host]

	return isDenied
}

func isAuthenticated(r *http.Request) bool {
	u, ok := request.UserFrom(r.Context())
	if !ok {
		return false
	}
	for _, g := range u.GetGroups() {
		if g == "system:authenticated" {
			return true
		}
	}
	return false
}
