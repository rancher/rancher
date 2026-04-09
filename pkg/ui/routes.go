package ui

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/parse"
	"github.com/rancher/rancher/pkg/cacerts"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
)

func New(_ v3.PreferenceCache, clusterRegistrationTokenCache v3.ClusterRegistrationTokenCache) http.Handler {
	router := http.NewServeMux()

	router.Handle("/{$}", PreferredIndex())
	router.Handle("/cacerts", cacerts.Handler(clusterRegistrationTokenCache))
	router.Handle("/asset-manifest.json", ember.ServeAsset())
	router.Handle("/crossdomain.xml", ember.ServeAsset())
	router.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusFound))
	router.Handle("/humans.txt", ember.ServeAsset())
	router.Handle("/index.txt", ember.ServeAsset())
	router.Handle("/robots.txt", ember.ServeAsset())
	router.Handle("/VERSION.txt", ember.ServeAsset())
	router.Handle("/favicon.png", vue.ServeFaviconDashboard())
	router.Handle("/favicon.ico", vue.ServeFaviconDashboard())
	router.HandleFunc("/verify-auth-azure", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("state") {
			redirectAuth(w, r)
		} else {
			emberIndexUnlessAPI().ServeHTTP(w, r)
		}
	})
	router.HandleFunc("/verify-auth", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("state") {
			redirectAuth(w, r)
		} else {
			emberIndexUnlessAPI().ServeHTTP(w, r)
		}
	})
	router.Handle("/api-ui/", ember.ServeAPIUI())
	router.Handle("/assets/rancher-ui-driver-linode/", emberAlwaysOffline.ServeAsset())
	router.Handle("/assets/", ember.IndexFileOnNotFound())
	router.Handle("/assets", ember.IndexFileOnNotFound())
	router.Handle("/dashboard/", vue.IndexFileOnNotFound())
	router.Handle("/ember-fetch/", ember.ServeAsset())
	router.Handle("/engines-dist/", ember.ServeAsset())
	router.Handle("/static/", ember.ServeAsset())
	router.Handle("/translations/", ember.IndexFileOnNotFound())
	router.Handle("/translations", ember.IndexFileOnNotFound())
	router.Handle("/", emberIndexUnlessAPI())

	return router
}

func emberIndexUnlessAPI() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			emberIndex.ServeHTTP(rw, req)
		} else {
			http.NotFound(rw, req)
		}
	})
}

func PreferredIndex() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, "/dashboard/", http.StatusFound)
	})
}
