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
	router.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusFound))
	router.Handle("/humans.txt", vue.ServeAsset())
	router.Handle("/robots.txt", vue.ServeAsset())
	router.Handle("/VERSION.txt", vue.ServeAsset())
	router.Handle("/favicon.png", vue.ServeFaviconDashboard())
	router.Handle("/favicon.ico", vue.ServeFaviconDashboard())
	router.HandleFunc("/verify-auth-azure", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("state") {
			redirectAuth(w, r)
		} else {
			vueIndexUnlessAPI().ServeHTTP(w, r)
		}
	})
	router.HandleFunc("/verify-auth", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("state") {
			redirectAuth(w, r)
		} else {
			vueIndexUnlessAPI().ServeHTTP(w, r)
		}
	})
	router.Handle("/api-ui/", vue.ServeAPIUI())
	router.Handle("/dashboard/", vue.IndexFileOnNotFound())
	router.Handle("/", vueIndexUnlessAPI())

	return router
}

func vueIndexUnlessAPI() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			vueIndex.ServeHTTP(rw, req)
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
