package catalog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRequest_content_type(t *testing.T) {
	response := "var http = require('http');\n        var url = require('url');\n        var number = 0;"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=testing.js")
		if _, err := w.Write([]byte(response)); err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()
	denyFunc := func(string) bool { return false }
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	w := httptest.NewRecorder()

	proxyRequest(ts.URL, "/testing.js", w, req, denyFunc)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got StatusCode %v, want %v", resp.StatusCode, http.StatusOK)
	}

	wantContent := "text/javascript; charset=utf-8"
	if ct := resp.Header.Get("Content-Type"); ct != wantContent {
		t.Errorf("got Content-Type %s, want %s", ct, wantContent)

	}
	if string(body) != response {
		t.Errorf("read body: %s", body)
	}
}
