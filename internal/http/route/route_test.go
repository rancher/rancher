package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleSubtree(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	HandleSubtree(mux, "/v3", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("handled " + req.URL.Path))
	}))
	mux.Handle("/", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("fallthrough"))
	}))

	tests := []struct {
		requestPath string
		wantBody    string
	}{
		{requestPath: "/v3", wantBody: "handled /v3"},
		{requestPath: "/v3/", wantBody: "handled /v3/"},
		{requestPath: "/v3/clusters", wantBody: "handled /v3/clusters"},
		{requestPath: "/v1", wantBody: "fallthrough"},
	}

	for _, test := range tests {
		t.Run(test.requestPath, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.requestPath, nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Empty(t, recorder.Header().Get("Location"), "request must not be redirected")
			assert.Equal(t, test.wantBody, recorder.Body.String())
		})
	}
}
