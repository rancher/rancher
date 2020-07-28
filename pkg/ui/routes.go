package ui

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/parse"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
)

func New() http.Handler {
	router := mux.NewRouter()
	router.UseEncodedPath()

	router.Handle("/", PreferredIndex())
	router.Handle("/asset-manifest.json", ember.ServeAsset())
	router.Handle("/crossdomain.xml", ember.ServeAsset())
	router.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusFound))
	router.Handle("/humans.txt", ember.ServeAsset())
	router.Handle("/index.txt", ember.ServeAsset())
	router.Handle("/robots.txt", ember.ServeAsset())
	router.Handle("/VERSION.txt", ember.ServeAsset())
	router.PathPrefix("/api-ui").Handler(ember.ServeAsset())
	router.PathPrefix("/assets").Handler(ember.ServeAsset())
	router.PathPrefix("/dashboard").Handler(vue.IndexFile())
	router.PathPrefix("/ember-fetch").Handler(ember.ServeAsset())
	router.PathPrefix("/engines-dist").Handler(ember.ServeAsset())
	router.PathPrefix("/static").Handler(ember.ServeAsset())
	router.PathPrefix("/translations").Handler(ember.ServeAsset())
	router.NotFoundHandler = emberIndexUnlessNoMCM()

	return router
}

func emberIndexUnlessNoMCM() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if parse.IsBrowser(req, true) {
			if features.MCM.Enabled() {
				emberIndex.ServeHTTP(rw, req)
			} else {
				vueIndex.ServeHTTP(rw, req)
			}
		} else {
			http.NotFound(rw, req)
		}
	})
}

func PreferredIndex() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if settings.UIPreferred.Get() == "ember" && features.MCM.Enabled() {
			emberIndex.ServeHTTP(rw, req)
		} else {
			vueIndex.ServeHTTP(rw, req)
		}
	})
}
