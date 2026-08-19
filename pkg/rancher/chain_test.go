package rancher

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	steveapi "github.com/rancher/rancher/pkg/api/steve"
	"github.com/rancher/rancher/pkg/api/steve/projects"
	"github.com/rancher/rancher/pkg/api/steve/proxy"
	"github.com/rancher/rancher/pkg/auth"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/websocket"
	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// markerHandler returns a handler that writes its own name, so a test can tell
// which table in the chain answered a request.
func markerHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(name))
	})
}

func identityMW(h http.Handler) http.Handler {
	return h
}

// newTestMCM builds the real multi-cluster management table with marker
// handlers. Its middlewares are identity: which table owns a path is what this
// test covers, and the authentication wrappers are pinned by the table's own
// test in pkg/multiclustermanager.
func newTestMCM(next http.Handler) http.Handler {
	return multiclustermanager.NewMCMMux(multiclustermanager.MCMRoutes{
		Limit:      identityMW,
		SAAuthedMW: identityMW,
		AuthedMW:   identityMW,
		MetricsMW:  identityMW,

		K8sProxy:      markerHandler("mcm:k8sProxy"),
		ManagementAPI: markerHandler("mcm:managementAPI"),
		Connect:       markerHandler("mcm:connect"),
		ClusterImport: markerHandler("mcm:clusterImport"),
		Version:       markerHandler("mcm:version"),
		ChannelServer: markerHandler("mcm:channelServer"),
		SAML:          markerHandler("mcm:saml"),
		V3Public:      markerHandler("mcm:v3Public"),
		V1Public:      markerHandler("mcm:v1Public"),
		SCIM:          markerHandler("mcm:scim"),
		Metrics:       markerHandler("mcm:metrics"),
		MetaAKS:       markerHandler("mcm:metaAKS"),
		MetaGKE:       markerHandler("mcm:metaGKE"),
		MetaAlibaba:   markerHandler("mcm:metaAlibaba"),
		MetaOCI:       markerHandler("mcm:metaOCI"),
		MetaVsphere:   markerHandler("mcm:metaVsphere"),
		MetaProxy:     markerHandler("mcm:metaProxy"),
		TokenReview:   markerHandler("mcm:tokenReview"),
		SupportConfig: markerHandler("mcm:supportConfig"),
		TokenAPI:      markerHandler("mcm:tokenAPI"),
		Logout:        markerHandler("mcm:logout"),
		Next:          next,
	})
}

func newTestAuthServer(next http.Handler) http.Handler {
	return auth.NewRootMux(auth.AuthRoutes{
		Limit:    identityMW,
		SAML:     markerHandler("auth:saml"),
		V3Public: markerHandler("auth:v3Public"),
		V1Public: markerHandler("auth:v1Public"),
		SCIM:     markerHandler("auth:scim"),
		V3:       authV3Handler(next),
		Logout:   markerHandler("auth:logout"),
		Next:     next,
	})
}

// authV3Handler mirrors the dispatch newAPIManagement puts behind the auth
// server's /v3/ route: a handful of prefixes go to the token API or the Norman
// APIs and everything else carries on down the chain. It lives in
// pkg/auth/server.go outside newRootMux, so the chain test has to model it -
// treating /v3/ as one opaque handler would claim the auth server owns paths
// like /v3/connect, which it hands off instead.
func authV3Handler(next http.Handler) http.Handler {
	tokenAPI := markerHandler("auth:tokenAPI")
	normanAPI := markerHandler("auth:normanAPI")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/v3/identit"), strings.HasPrefix(path, "/v3/token"):
			tokenAPI.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/v3/authConfig"),
			strings.HasPrefix(path, "/v3/principal"),
			strings.HasPrefix(path, "/v3/user"),
			strings.HasPrefix(path, "/v3/schema"),
			strings.HasPrefix(path, "/v3/subscribe"):
			normanAPI.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func newTestPreMCM(next http.Handler) http.Handler {
	return steveapi.NewPreMCMMux(steveapi.PreMCMRoutes{
		ConfigServer: markerHandler("preMCM:configServer"),
		Installer:    markerHandler("preMCM:installer"),
		Next:         next,
	})
}

// newTestAdditionalAPI builds the extras table over the projects table, the
// same nesting AdditionalAPIs sets up with nextHandler = clusterAPI(next).
func newTestAdditionalAPI(next http.Handler) http.Handler {
	return steveapi.NewAdditionalAPIMux(steveapi.AdditionalAPIRoutes{
		Github: markerHandler("extras:github"),
		Tunnel: markerHandler("extras:tunnel"),
		RegisterUIPlugins: func(mux *http.ServeMux) {
			mux.Handle("/v1/uiplugins", markerHandler("extras:uiplugins"))
		},
		Next: projects.NewProjectsMux(markerHandler("projects:server"), next),
	})
}

// newTestChain assembles the real chain. Cluster proxy and aggregation go in as
// identity: the test covers ownership among the extracted tables and treats
// those two as transparent.
//
// mcmEnabled false is modelled as identity in the mcm slot. That is what
// production does with MCM off: wranglerContext.MultiClusterManager is a
// *DeferredServer whatever the feature flag says, and its Middleware falls
// straight through to next because Start never ran, so getMCM returns nil. The
// noopMCM in pkg/wrangler is only the zero value and never reaches the request
// path.
func newTestChain(mcmEnabled bool) http.Handler {
	mcm := identityMW
	if mcmEnabled {
		mcm = newTestMCM
	}

	return newHandlerChain(chainHandlers{
		setAuthHeader:       auth.SetXAPICattleAuthHeader,
		contentTypeOptions:  responsewriter.ContentTypeOptions,
		noCache:             responsewriter.NoCache,
		websocket:           websocket.NewWebsocketHandler,
		rewriteLocalCluster: proxy.RewriteLocalCluster,
		clusterProxy:        identityMW,
		aggregation:         identityMW,
		preMCM:              newTestPreMCM,
		mcm:                 mcm,
		authServer:          newTestAuthServer,
		additionalAPI:       newTestAdditionalAPI,
		requireAuthed:       requests.NewRequireAuthenticatedFilter("/v1/", "/v1/management.cattle.io.setting"),
		steve:               markerHandler("steve"),
	})
}

// TestHandlerChainOwnership pins which table answers each path, with MCM on and
// off. Paths whose owner changes between the two modes are the ones a route
// change can break in one mode only, which is what #56580 was.
func TestHandlerChainOwnership(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method    string
		path      string
		wantMCMOn string
		// wantMCMOff is the owner with MCM disabled. Equal to wantMCMOn for
		// every path that is not registered twice.
		wantMCMOff string
		// authenticated sends the request with a user in its context, for the
		// routes registered behind an authenticated filter.
		authenticated bool
	}{
		// The pre-MCM table sits above everything and owns its paths in both
		// modes.
		{method: http.MethodGet, path: "/v3/connect/agent", wantMCMOn: "preMCM:configServer", wantMCMOff: "preMCM:configServer"},
		{method: http.MethodGet, path: "/v3/connect/config-yaml", wantMCMOn: "preMCM:configServer", wantMCMOff: "preMCM:configServer"},
		{method: http.MethodGet, path: "/system-agent-install.sh", wantMCMOn: "preMCM:installer", wantMCMOff: "preMCM:installer"},

		// Registered in both the MCM table and the auth server. MCM wins when
		// it is on. Branch B removes the MCM copies, which flips the left
		// column to match the right one.
		{method: http.MethodGet, path: "/v1-saml/login", wantMCMOn: "mcm:saml", wantMCMOff: "auth:saml"},
		{method: http.MethodGet, path: "/v3-public/authProviders", wantMCMOn: "mcm:v3Public", wantMCMOff: "auth:v3Public"},
		{method: http.MethodGet, path: "/v1-public/authProviders", wantMCMOn: "mcm:v1Public", wantMCMOff: "auth:v1Public"},
		{method: http.MethodGet, path: "/v1-scim/v2/Users", wantMCMOn: "mcm:scim", wantMCMOff: "auth:scim"},
		// The auth server registers logout behind an authenticated filter, so
		// this case needs a user to reach the handler at all.
		{method: http.MethodPost, path: "/v1/logout", wantMCMOn: "mcm:logout", wantMCMOff: "auth:logout", authenticated: true},

		// Registered in both the MCM table and the extras table. The auth
		// server's /v3/ route sits between them and hands this path on.
		{method: http.MethodGet, path: "/v3/connect", wantMCMOn: "mcm:connect", wantMCMOff: "extras:tunnel"},

		// Owned by MCM alone. With MCM off they fall through to the terminal
		// handler, which is the shape #56580 had.
		{method: http.MethodGet, path: "/v3/connect/register", wantMCMOn: "mcm:connect", wantMCMOff: "steve"},
		{method: http.MethodGet, path: "/metrics", wantMCMOn: "mcm:metrics", wantMCMOff: "steve"},
		{method: http.MethodGet, path: "/rancherversion", wantMCMOn: "mcm:version", wantMCMOff: "steve"},
		{method: http.MethodGet, path: "/v1-k3s-release/channels", wantMCMOn: "mcm:channelServer", wantMCMOff: "steve"},
		{method: http.MethodGet, path: "/meta/proxy/example.com", wantMCMOn: "mcm:metaProxy", wantMCMOff: "steve"},
		{method: http.MethodGet, path: "/k8s/clusters/c-abcde/version", wantMCMOn: "mcm:k8sProxy", wantMCMOff: "steve"},

		// /v3/ is not a duplicate: the two tables serve different endpoints
		// behind the same prefix. The management API is MCM's alone, tokens are
		// served in both modes, and the rest of /v3/ carries on down the chain
		// when MCM is off.
		{method: http.MethodGet, path: "/v3/clusters", wantMCMOn: "mcm:managementAPI", wantMCMOff: "steve"},
		{method: http.MethodGet, path: "/v3/tokens", wantMCMOn: "mcm:tokenAPI", wantMCMOff: "auth:tokenAPI"},
		{method: http.MethodGet, path: "/v3/users", wantMCMOn: "mcm:managementAPI", wantMCMOff: "auth:normanAPI"},

		// The extras table sits below both, so its paths are unaffected by the
		// mode.
		{method: http.MethodGet, path: "/v1/github/repos", wantMCMOn: "extras:github", wantMCMOff: "extras:github"},
		{method: http.MethodGet, path: "/v1/uiplugins", wantMCMOn: "extras:uiplugins", wantMCMOff: "extras:uiplugins"},
		{method: http.MethodGet, path: "/healthz", wantMCMOn: "ok", wantMCMOff: "ok"},
		{method: http.MethodGet, path: "/ping", wantMCMOn: "pong", wantMCMOff: "pong"},

		// The projects table sits below the extras mux.
		{method: http.MethodGet, path: "/v1/management.cattle.io.clusters/c-abcde?link=projects", wantMCMOn: "projects:server", wantMCMOff: "projects:server"},

		// Anything no table claims reaches the terminal handler.
		{method: http.MethodGet, path: "/dashboard/", wantMCMOn: "steve", wantMCMOff: "steve"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.wantMCMOn, serveChain(newTestChain(true), test.method, test.path, test.authenticated), "MCM enabled")
			assert.Equal(t, test.wantMCMOff, serveChain(newTestChain(false), test.method, test.path, test.authenticated), "MCM disabled")
		})
	}
}

// TestHandlerChainRequireAuthenticated pins the filter at the bottom of the
// chain: paths under /v1/ that no table claims need a login before they reach
// the terminal handler, apart from the settings exception.
func TestHandlerChainRequireAuthenticated(t *testing.T) {
	t.Parallel()

	chain := newTestChain(true)

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/pods", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	assert.Equal(t, "steve", serveChain(chain, http.MethodGet, "/v1/pods", true))
	assert.Equal(t, "steve", serveChain(chain, http.MethodGet, "/v1/management.cattle.io.setting", false))
}

func serveChain(h http.Handler, method, path string, authenticated bool) string {
	req := httptest.NewRequest(method, path, nil)
	if authenticated {
		req = req.WithContext(request.WithUser(req.Context(),
			&user.DefaultInfo{Name: "u-abcde", Groups: []string{"system:authenticated"}}))
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Body.String()
}
