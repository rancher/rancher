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
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=testing.js")
		if _, err := w.Write([]byte(response)); err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()

	denyFunc := func(string) (bool, []netip.Addr) {
		return false, []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	}

	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	w := httptest.NewRecorder()

	proxyRequest(ts.URL, "/testing.js", w, req, denyFunc, ts.Client().Transport)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got StatusCode %v, want %v", resp.StatusCode, http.StatusOK)
	}
	wantContent := mime.TypeByExtension(".js")
	if ct := resp.Header.Get("Content-Type"); ct != wantContent {
		t.Errorf("got Content-Type %s, want %s", ct, wantContent)
	}
	if string(body) != response {
		t.Errorf("read body: %s", body)
	}
}
