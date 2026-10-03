package catalog

import (
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestProxyRequest_content_type(t *testing.T) {
	response := "var http = require('http');\n        var url = require('url');\n        var number = 0;"

	tests := []struct {
		name                string
		path                string
		upstreamContentType string
		wantContentType     string
	}{
		{
			name:            "known extension",
			path:            "/testing.js",
			wantContentType: mime.TypeByExtension(".js"),
		},
		{
			name:                "known extension overrides upstream content type",
			path:                "/testing.js",
			upstreamContentType: "text/html",
			wantContentType:     mime.TypeByExtension(".js"),
		},
		{
			name:            "unknown extension",
			path:            "/testing",
			wantContentType: http.DetectContentType([]byte(response)),
		},
		{
			name:                "unknown extension overrides upstream content type",
			path:                "/testing",
			upstreamContentType: "text/html",
			wantContentType:     http.DetectContentType([]byte(response)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Disposition", "attachment; filename=testing.js")
				if tt.upstreamContentType != "" {
					w.Header().Set("Content-Type", tt.upstreamContentType)
				}
				if _, err := w.Write([]byte(response)); err != nil {
					t.Fatal(err)
				}
			}))
			defer ts.Close()

			denyFunc := func(string) (bool, []netip.Addr) {
				return false, []netip.Addr{netip.MustParseAddr("127.0.0.1")}
			}

			req := httptest.NewRequest(http.MethodGet, "https://example.com"+tt.path, nil)
			w := httptest.NewRecorder()

			proxyRequest(ts.URL, tt.path, w, req, denyFunc, ts.Client().Transport)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("got StatusCode %v, want %v", resp.StatusCode, http.StatusOK)
			}
			// Compare the full slice rather than using Header.Get, which only
			// returns the first value and would hide a duplicated header.
			if ct := resp.Header.Values("Content-Type"); len(ct) != 1 || ct[0] != tt.wantContentType {
				t.Errorf("got Content-Type %q, want [%q]", ct, tt.wantContentType)
			}
			if string(body) != response {
				t.Errorf("read body: %s", body)
			}
		})
	}
}
