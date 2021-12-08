package ui

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/parse"
	"github.com/rancher/rancher/pkg/cacerts"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
)

func New(_ v3.PreferenceCache, clusterRegistrationTokenCache v3.ClusterRegistrationTokenCache) http.Handler {
	router := mux.NewRouter()
	router.UseEncodedPath()

	router.Handle("/", PreferredIndex())
	router.Handle("/cacerts", cacerts.Handler(clusterRegistrationTokenCache))
	router.Handle("/asset-manifest.json", ember.ServeAsset())
	router.Handle("/crossdomain.xml", ember.ServeAsset())
	router.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusFound))
	router.Handle("/dashboard/", vue.IndexFile())
	router.Handle("/humans.txt", ember.ServeAsset())
	router.Handle("/index.txt", ember.ServeAsset())
	router.Handle("/robots.txt", ember.ServeAsset())
	router.Handle("/VERSION.txt", ember.ServeAsset())
	router.Handle("/favicon.png", vue.ServeFaviconDashboard())
	router.Handle("/favicon.ico", vue.ServeFaviconDashboard())
	router.Path("/verify-auth-azure").Queries("state", "{state}").HandlerFunc(redirectAuth)
	router.Path("/verify-auth").Queries("state", "{state}").HandlerFunc(redirectAuth)
	router.PathPrefix("/api-ui").Handler(ember.ServeAsset())
	router.PathPrefix("/assets/rancher-ui-driver-linode").Handler(emberAlwaysOffline.ServeAsset())
	router.PathPrefix("/assets").Handler(ember.ServeAsset())
	router.PathPrefix("/dashboard/").Handler(vue.IndexFileOnNotFound())
	router.PathPrefix("/ember-fetch").Handler(ember.ServeAsset())
	router.PathPrefix("/engines-dist").Handler(ember.ServeAsset())
	router.PathPrefix("/static").Handler(ember.ServeAsset())
	router.PathPrefix("/translations").Handler(ember.ServeAsset())
	router.NotFoundHandler = emberIndexUnlessAPI()

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
