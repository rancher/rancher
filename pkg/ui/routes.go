package ui

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/parse"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/slice"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func New(prefs v3.PreferenceCache) http.Handler {
	router := mux.NewRouter()
	router.UseEncodedPath()

	router.Handle("/", PreferredIndex(prefs))
	router.Handle("/asset-manifest.json", ember.ServeAsset())
	router.Handle("/crossdomain.xml", ember.ServeAsset())
	router.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusFound))
	router.Handle("/dashboard/", vue.IndexFile())
	router.Handle("/humans.txt", ember.ServeAsset())
	router.Handle("/index.txt", ember.ServeAsset())
	router.Handle("/robots.txt", ember.ServeAsset())
	router.Handle("/VERSION.txt", ember.ServeAsset())
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

func PreferredIndex(prefs v3.PreferenceCache) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		user, ok := request.UserFrom(req.Context())
		if !ok || !slice.ContainsString(user.GetGroups(), "system:authenticated") {
			serveIndexFromSetting(rw, req, settings.UIPreferred.Get())
			return
		}

		pref, err := prefs.Get(user.GetName(), "landing")
		if err == nil && pref.Value != "" {
			serveIndexFromSetting(rw, req, pref.Value)
			return
		}

		serveIndexFromSetting(rw, req, settings.UIDefaultLanding.Get())
	})
}

func serveIndexFromSetting(rw http.ResponseWriter, req *http.Request, setting string) {
	if strings.Contains(setting, "ember") {
		emberIndex.ServeHTTP(rw, req)
	} else {
		http.Redirect(rw, req, "/dashboard/", http.StatusFound)
	}
}
