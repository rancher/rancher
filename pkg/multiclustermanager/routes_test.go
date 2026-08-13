package multiclustermanager

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/api/steve/supportconfigs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markerHandler writes its name so tests can tell which route was picked.
func markerHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(name))
	})
}

// identityMW passes the handler through unchanged, so tests that care about
// which route was picked are not affected by the middleware stacks.
func identityMW(h http.Handler) http.Handler {
	return h
}

func testMCMRoutes() MCMRoutes {
	return MCMRoutes{
		Limit:      identityMW,
		SAAuthedMW: identityMW,
		AuthedMW:   identityMW,
		MetricsMW:  identityMW,

		K8sProxy:      markerHandler("k8sProxy"),
		ManagementAPI: markerHandler("managementAPI"),
		Connect:       markerHandler("connect"),
		ClusterImport: markerHandler("clusterImport"),
		Version:       markerHandler("version"),
		ChannelServer: markerHandler("channelServer"),
		SAML:          markerHandler("saml"),
		V1Public:      markerHandler("v1Public"),
		Metrics:       markerHandler("metrics"),
		MetaAKS:       markerHandler("metaAKS"),
		MetaGKE:       markerHandler("metaGKE"),
		MetaAlibaba:   markerHandler("metaAlibaba"),
		MetaOCI:       markerHandler("metaOCI"),
		MetaVsphere:   markerHandler("metaVsphere"),
		MetaProxy:     markerHandler("metaProxy"),
		TokenReview:   markerHandler("tokenReview"),
		SupportConfig: markerHandler("supportConfig"),
		TokenAPI:      markerHandler("tokenAPI"),
		Logout:        markerHandler("logout"),
		Next:          markerHandler("next"),
	}
}

func TestNewMCMMux(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		path     string
		header   map[string]string
		wantCode int
		wantBody string
	}{
		{
			name:     "the root path serves the management API to a non browser",
			method:   http.MethodGet,
			path:     "/",
			header:   map[string]string{"User-Agent": "curl/8.7.1", "Accept": "*/*"},
			wantBody: "managementAPI",
		},
		{
			name:   "the root path falls through for a browser",
			method: http.MethodGet,
			path:   "/",
			// A browser is a Mozilla user agent that accepts */*. Both halves
			// are required, so this case sets both.
			header:   map[string]string{"User-Agent": "Mozilla/5.0", "Accept": "*/*"},
			wantBody: "next",
		},
		{
			name:     "a request with no headers is not a browser",
			method:   http.MethodGet,
			path:     "/",
			wantBody: "managementAPI",
		},
		{
			name:     "cluster proxy is served",
			method:   http.MethodGet,
			path:     "/k8s/clusters/c-abcde/v1/pods",
			wantBody: "k8sProxy",
		},
		{
			name:     "k8s proxy is served",
			method:   http.MethodGet,
			path:     "/k8s/proxy/c-abcde/v1/pods",
			wantBody: "k8sProxy",
		},
		{
			name:     "connect is served",
			method:   http.MethodGet,
			path:     "/v3/connect",
			wantBody: "connect",
		},
		{
			name:     "connect register is served",
			method:   http.MethodGet,
			path:     "/v3/connect/register",
			wantBody: "connect",
		},
		{
			name:     "a well formed import filename is served",
			method:   http.MethodGet,
			path:     "/v3/import/sometoken_c-abcde.yaml",
			wantBody: "clusterImport",
		},
		{
			name:     "an import filename without a token is not found",
			method:   http.MethodGet,
			path:     "/v3/import/c-abcde.yaml",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "an import filename that is not yaml is not found",
			method:   http.MethodGet,
			path:     "/v3/import/sometoken_c-abcde.json",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "the cacerts setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/cacerts",
			wantBody: "managementAPI",
		},
		{
			name:     "the first-login setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/first-login",
			wantBody: "managementAPI",
		},
		{
			name:     "the ui-banners setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/ui-banners",
			wantBody: "managementAPI",
		},
		{
			name:     "the ui-issues setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/ui-issues",
			wantBody: "managementAPI",
		},
		{
			name:     "the ui-pl setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/ui-pl",
			wantBody: "managementAPI",
		},
		{
			name:     "the ui-brand setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/ui-brand",
			wantBody: "managementAPI",
		},
		{
			name:     "the ui-default-landing setting is served unauthenticated",
			method:   http.MethodGet,
			path:     "/v3/settings/ui-default-landing",
			wantBody: "managementAPI",
		},
		{
			name:     "a setting that is not on the unauthenticated list falls through to the authenticated table",
			method:   http.MethodGet,
			path:     "/v3/settings/server-url",
			wantBody: "managementAPI",
		},
		{
			name:     "posting to an unauthenticated setting falls through to the authenticated table",
			method:   http.MethodPost,
			path:     "/v3/settings/cacerts",
			wantBody: "managementAPI",
		},
		{
			name:     "the version is served",
			method:   http.MethodGet,
			path:     "/rancherversion",
			wantBody: "version",
		},
		{
			name:     "k3s releases are served",
			method:   http.MethodGet,
			path:     "/v1-k3s-release/channels",
			wantBody: "channelServer",
		},
		{
			name:     "rke2 releases are served",
			method:   http.MethodGet,
			path:     "/v1-rke2-release/channels",
			wantBody: "channelServer",
		},
		{
			name:     "saml is served",
			method:   http.MethodGet,
			path:     "/v1-saml/adfs/login",
			wantBody: "saml",
		},
		{
			name:     "v1 public is served",
			method:   http.MethodGet,
			path:     "/v1-public/authProviders",
			wantBody: "v1Public",
		},
		{
			name:     "the aks meta resource is served",
			method:   http.MethodGet,
			path:     "/meta/aksVirtualNetworks",
			wantBody: "metaAKS",
		},
		{
			name:     "the gke meta resource is served",
			method:   http.MethodGet,
			path:     "/meta/gkeNetworks",
			wantBody: "metaGKE",
		},
		{
			name:     "the alibaba meta resource is served",
			method:   http.MethodGet,
			path:     "/meta/alibabaInstanceTypes",
			wantBody: "metaAlibaba",
		},
		{
			name:     "an unknown meta resource falls through to the metrics table",
			method:   http.MethodGet,
			path:     "/meta/somethingelse",
			wantBody: "next",
		},
		{
			name:     "the oci meta resource is served",
			method:   http.MethodGet,
			path:     "/meta/oci/vcns",
			wantBody: "metaOCI",
		},
		{
			name:     "the vsphere meta field is served",
			method:   http.MethodGet,
			path:     "/meta/vsphere/datacenter",
			wantBody: "metaVsphere",
		},
		{
			name:     "a non GET vsphere meta field falls through",
			method:   http.MethodPost,
			path:     "/meta/vsphere/datacenter",
			wantBody: "next",
		},
		{
			name:     "the meta proxy is served",
			method:   http.MethodGet,
			path:     "/meta/proxy/example.com/path",
			wantBody: "metaProxy",
		},
		{
			name:     "token review is served",
			method:   http.MethodPost,
			path:     "/v3/tokenreview",
			wantBody: "tokenReview",
		},
		{
			// /v3/tokenreview shares the /v3/token prefix the token API claims,
			// so a GET lands there rather than on the management API.
			name:     "a non POST token review falls through to the token API",
			method:   http.MethodGet,
			path:     "/v3/tokenreview",
			wantBody: "tokenAPI",
		},
		{
			name:     "the support config endpoint is served",
			method:   http.MethodGet,
			path:     supportconfigs.Endpoint,
			wantBody: "supportConfig",
		},
		{
			name:     "tokens are served by the token API",
			method:   http.MethodGet,
			path:     "/v3/tokens",
			wantBody: "tokenAPI",
		},
		{
			name:     "identities are served by the token API",
			method:   http.MethodGet,
			path:     "/v3/identities",
			wantBody: "tokenAPI",
		},
		{
			name:     "other v3 paths are served by the management API",
			method:   http.MethodGet,
			path:     "/v3/clusters",
			wantBody: "managementAPI",
		},
		{
			name:     "logout is served",
			method:   http.MethodPost,
			path:     "/v1/logout",
			wantBody: "logout",
		},
		{
			name:     "a non POST logout falls through",
			method:   http.MethodGet,
			path:     "/v1/logout",
			wantBody: "next",
		},
		{
			name:     "metrics are served",
			method:   http.MethodGet,
			path:     "/metrics",
			wantBody: "metrics",
		},
		{
			name:     "an unclaimed path falls through",
			method:   http.MethodGet,
			path:     "/v1/management.cattle.io.settings",
			wantBody: "next",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(test.method, test.path, nil)
			for k, v := range test.header {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			NewMCMMux(testMCMRoutes()).ServeHTTP(rec, req)

			wantCode := test.wantCode
			if wantCode == 0 {
				wantCode = http.StatusOK
			}
			require.Equal(t, wantCode, rec.Code)
			if test.wantBody != "" {
				assert.Equal(t, test.wantBody, rec.Body.String())
			}
		})
	}
}

func TestNewMCMMuxOptionalRoutes(t *testing.T) {
	t.Parallel()

	t.Run("v3 public and scim fall through when not configured", func(t *testing.T) {
		t.Parallel()

		mux := NewMCMMux(testMCMRoutes())

		for _, path := range []string{"/v3-public/authProviders", "/v1-scim/v2/Users"} {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			assert.Equal(t, "next", rec.Body.String(), "path %s", path)
		}
	})

	t.Run("v3 public and scim are served when configured", func(t *testing.T) {
		t.Parallel()

		routes := testMCMRoutes()
		routes.V3Public = markerHandler("v3Public")
		routes.SCIM = markerHandler("scim")
		mux := NewMCMMux(routes)

		for path, want := range map[string]string{
			"/v3-public/authProviders": "v3Public",
			"/v1-scim/v2/Users":        "scim",
		} {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			assert.Equal(t, want, rec.Body.String(), "path %s", path)
		}
	})
}

// TestNewMCMMuxMiddleware pins which middleware stack owns which route. Each
// wrapper tags the response with its own name, so the body reads as the stack
// that ran followed by the handler that answered.
func TestNewMCMMuxMiddleware(t *testing.T) {
	t.Parallel()

	tagging := func(name string) func(http.Handler) http.Handler {
		return func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(name + ":"))
				h.ServeHTTP(w, r)
			})
		}
	}

	routes := testMCMRoutes()
	routes.Limit = tagging("limit")
	routes.SAAuthedMW = tagging("saAuthed")
	routes.AuthedMW = tagging("authed")
	routes.MetricsMW = tagging("metricsMW")
	mux := NewMCMMux(routes)

	tests := []struct {
		name     string
		method   string
		path     string
		wantBody string
	}{
		{
			name:     "unauthenticated routes are limited only",
			method:   http.MethodGet,
			path:     "/rancherversion",
			wantBody: "limit:version",
		},
		{
			name:     "the service account table wraps the cluster proxy",
			method:   http.MethodGet,
			path:     "/k8s/clusters/c-abcde/v1/pods",
			wantBody: "limit:saAuthed:k8sProxy",
		},
		{
			name:     "the authenticated table wraps logout",
			method:   http.MethodPost,
			path:     "/v1/logout",
			wantBody: "limit:authed:logout",
		},
		{
			name:     "the metrics table wraps metrics",
			method:   http.MethodGet,
			path:     "/metrics",
			wantBody: "limit:metricsMW:metrics",
		},
		{
			name:     "the fallthrough is limited only",
			method:   http.MethodGet,
			path:     "/v1/management.cattle.io.settings",
			wantBody: "limit:next",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(test.method, test.path, nil))

			assert.Equal(t, test.wantBody, rec.Body.String())
		})
	}
}
