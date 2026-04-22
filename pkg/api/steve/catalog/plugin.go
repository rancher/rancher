package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	neturl "net/url"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/controllers/dashboard/plugin"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type denyFunc func(host string) (bool, []netip.Addr)

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
		return
	}
	if _, err := w.Write(index); err != nil {
		logrus.Error(err)
	}
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
		if entry.Endpoint != "" {
			logrus.Debugf("[noCache: %v] proxying request to [endpoint: %v]\n", entry.NoCache, entry.Endpoint)
			proxyRequest(entry.Endpoint, vars["rest"], w, r, denylist, nil)
		} else {
			logrus.Errorf("[noCache: %v] caching still in progress for [endpoint: %v]\n", entry.NoCache, entry.Endpoint)
			http.Error(w, "caching still in progress", http.StatusTooEarly)
		}
	} else {
		logrus.Debugf("[noCache: %v] serving plugin files from filesystem cache\n", entry.NoCache)
		r.URL.Path = fmt.Sprintf("/%s/%s/%s", vars["name"], vars["version"], vars["rest"])
		http.FileServer(http.Dir(plugin.FSCacheRootDir)).ServeHTTP(w, r)
	}
}

func proxyRequest(target, path string, w http.ResponseWriter, r *http.Request, denyListFunc denyFunc, transport http.RoundTripper) {
	url, err := neturl.Parse(target)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse url [%s]", target), http.StatusInternalServerError)
		return
	}

	if url.Scheme != "https" {
		http.Error(w, fmt.Sprintf("url scheme [%s] is not allowed, only https is permitted", url.Scheme), http.StatusForbidden)
		return
	}

	denied, resolvedAddrs := denyListFunc(url.Hostname())
	if denied {
		http.Error(w, fmt.Sprintf("url [%s] is forbidden", target), http.StatusForbidden)
		return
	}

	port := url.Port()
	if port == "" {
		port = "443"
	}
	pinnedAddrs := resolvedAddrs
	if transport == nil {
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var lastErr error
				for _, addr := range pinnedAddrs {
					dialer := &net.Dialer{}
					conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, fmt.Errorf("failed to connect to any resolved address: %w", lastErr)
			},
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = transport
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

func denylist(host string) (bool, []netip.Addr) {
	if host == "" {
		return true, nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		logrus.Debugf("denylist: failed to resolve host %s: %v", host, err)
		return true, nil
	}

	if len(ips) == 0 {
		return true, nil
	}

	var allowed []netip.Addr
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return true, nil
		}
		addr = addr.Unmap()
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
			return true, nil
		}
		allowed = append(allowed, addr)
	}

	return false, allowed
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
