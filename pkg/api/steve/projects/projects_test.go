package projects

import (
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestNewProjectsMux(t *testing.T) {
	t.Parallel()

	mux := NewProjectsMux(markerHandler("server"), markerHandler("next"))

	tests := []struct {
		method string
		path   string
		want   string
	}{
		// The cluster path is shared with the rest of the chain: only the
		// project links belong to the projects server.
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde?link=projects", "server"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde?link=project", "server"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde?link=schemas", "next"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde?link=subscribe", "next"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde", "next"},
		{http.MethodPost, "/v1/management.cattle.io.clusters/c-abcde?link=projects", "server"},

		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde/project", "server"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde/project/p-abcde", "server"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde/project/ns/p-abcde", "server"},
		{http.MethodPut, "/v1/management.cattle.io.clusters/c-abcde/project/p-abcde", "server"},

		{http.MethodGet, "/v1/management.cattle.io.clusters", "next"},
		{http.MethodGet, "/v1/management.cattle.io.clusters/c-abcde/project/ns/p-abcde/extra", "next"},
		{http.MethodGet, "/v1/pods", "next"},
		{http.MethodGet, "/v3/clusters", "next"},
		{http.MethodGet, "/", "next"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, serve(t, mux, test.method, test.path))
		})
	}
}

func TestPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		clusterID  string
		namespace  string
		wantPrefix string
	}{
		{
			name:       "cluster id wins",
			clusterID:  "c-abcde",
			namespace:  "ns",
			wantPrefix: "/v1/management.cattle.io.clusters/c-abcde",
		},
		{
			name:       "cluster id only",
			clusterID:  "c-abcde",
			wantPrefix: "/v1/management.cattle.io.clusters/c-abcde",
		},
		{
			name:       "namespace only",
			namespace:  "c-fghij",
			wantPrefix: "/v1/management.cattle.io.clusters/c-fghij",
		},
		{
			name: "neither set",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var got string
			h := prefix(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.PathValue("prefix")
			}))

			req := httptest.NewRequest(http.MethodGet, "/v1/management.cattle.io.clusters/x", nil)
			if test.clusterID != "" {
				req.SetPathValue("clusterID", test.clusterID)
			}
			if test.namespace != "" {
				req.SetPathValue("namespace", test.namespace)
			}

			h.ServeHTTP(httptest.NewRecorder(), req)
			assert.Equal(t, test.wantPrefix, got)
		})
	}
}
