package steve

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/capr/configserver"
	"github.com/rancher/rancher/pkg/capr/installer"
	"github.com/rancher/rancher/pkg/features"
	"github.com/stretchr/testify/assert"
)

// markerHandler returns a handler that writes its own name, so a test can tell
// which registration answered a request.
func markerHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(name))
	})
}

func serve(t *testing.T, h http.Handler, method, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec.Body.String()
}

func TestNewPreMCMMux(t *testing.T) {
	t.Parallel()

	mux := NewPreMCMMux(PreMCMRoutes{
		ConfigServer: markerHandler("configServer"),
		Installer:    markerHandler("installer"),
		Next:         markerHandler("next"),
	})

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, configserver.ConnectAgent, "configServer"},
		{http.MethodPost, configserver.ConnectAgent, "configServer"},
		{http.MethodGet, configserver.ConnectConfigYamlPath, "configServer"},
		{http.MethodGet, configserver.ConnectClusterInfo, "configServer"},
		{http.MethodGet, installer.SystemAgentInstallPath, "installer"},
		{http.MethodGet, installer.WindowsRke2InstallPath, "installer"},

		// The config server paths are exact matches, so anything below them
		// falls through rather than being served here.
		{http.MethodGet, configserver.ConnectAgent + "/extra", "next"},
		{http.MethodGet, "/v3/connect", "next"},
		{http.MethodGet, "/v1/github/foo", "next"},
		{http.MethodGet, "/", "next"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, serve(t, mux, test.method, test.path))
		})
	}
}

// TestAdditionalAPIsPreMCMRKE2Disabled asserts that with RKE2 off the returned
// middleware is bare identity: traffic reaches next unchanged. That is a
// different claim from "the routes are absent" - no mux is built at all.
func TestAdditionalAPIsPreMCMRKE2Disabled(t *testing.T) {
	features.RKE2.Set(false)
	t.Cleanup(features.RKE2.Unset)

	next := markerHandler("next")
	h := AdditionalAPIsPreMCM(nil)(next)

	assert.Equal(t, "next", serve(t, h, http.MethodGet, configserver.ConnectAgent))
	assert.Equal(t, "next", serve(t, h, http.MethodGet, installer.SystemAgentInstallPath))
}

func TestNewAdditionalAPIMux(t *testing.T) {
	t.Parallel()

	mux := NewAdditionalAPIMux(AdditionalAPIRoutes{
		Github: markerHandler("github"),
		Tunnel: markerHandler("tunnel"),
		Next:   markerHandler("next"),
	})

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/v1/github/foo", "github"},
		{http.MethodGet, "/v1/github/foo/bar/baz", "github"},
		{http.MethodPost, "/v1/github/foo", "github"},
		{http.MethodGet, "/v3/connect", "tunnel"},

		// Registered by health.Register, which is unconditional.
		{http.MethodGet, "/healthz", "ok"},
		{http.MethodGet, "/ping", "pong"},

		// /v3/connect is an exact match, so the config server paths registered
		// by the pre-MCM mux fall through here.
		{http.MethodGet, configserver.ConnectAgent, "next"},
		{http.MethodGet, "/v3/", "next"},
		{http.MethodGet, "/", "next"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, serve(t, mux, test.method, test.path))
		})
	}

	// The {path...} wildcard makes the mux redirect the bare prefix to the
	// trailing slash form rather than passing it along.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/github", nil))
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "/v1/github/", rec.Header().Get("Location"))
}

func TestNewAdditionalAPIMuxOptionalRoutes(t *testing.T) {
	t.Parallel()

	registerUIPlugins := func(mux *http.ServeMux) {
		mux.Handle("/v1/uiplugins", markerHandler("uiplugins"))
		mux.Handle("/v1/uiplugins/{name}/{version}/{rest...}", markerHandler("uiplugins"))
	}
	registerOIDC := func(mux *http.ServeMux) {
		mux.Handle("/oidc/token", markerHandler("oidc"))
		mux.Handle("/oidc/authorize", markerHandler("oidc"))
	}

	tests := []struct {
		name              string
		registerUIPlugins func(*http.ServeMux)
		registerOIDC      func(*http.ServeMux)
		wantUIPlugins     string
		wantOIDC          string
	}{
		{
			name:          "both disabled",
			wantUIPlugins: "next",
			wantOIDC:      "next",
		},
		{
			name:              "ui extension enabled",
			registerUIPlugins: registerUIPlugins,
			wantUIPlugins:     "uiplugins",
			wantOIDC:          "next",
		},
		{
			name:          "oidc provider enabled",
			registerOIDC:  registerOIDC,
			wantUIPlugins: "next",
			wantOIDC:      "oidc",
		},
		{
			name:              "both enabled",
			registerUIPlugins: registerUIPlugins,
			registerOIDC:      registerOIDC,
			wantUIPlugins:     "uiplugins",
			wantOIDC:          "oidc",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mux := NewAdditionalAPIMux(AdditionalAPIRoutes{
				Github:            markerHandler("github"),
				Tunnel:            markerHandler("tunnel"),
				Next:              markerHandler("next"),
				RegisterUIPlugins: test.registerUIPlugins,
				RegisterOIDC:      test.registerOIDC,
			})

			assert.Equal(t, test.wantUIPlugins, serve(t, mux, http.MethodGet, "/v1/uiplugins"))
			assert.Equal(t, test.wantUIPlugins, serve(t, mux, http.MethodGet, "/v1/uiplugins/name/1.0.0/plugin.js"))
			assert.Equal(t, test.wantOIDC, serve(t, mux, http.MethodGet, "/oidc/token"))
			assert.Equal(t, test.wantOIDC, serve(t, mux, http.MethodGet, "/oidc/authorize"))

			// The gates do not move the routes that are always registered.
			assert.Equal(t, "github", serve(t, mux, http.MethodGet, "/v1/github/foo"))
			assert.Equal(t, "tunnel", serve(t, mux, http.MethodGet, "/v3/connect"))
		})
	}
}
