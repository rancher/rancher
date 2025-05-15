package version

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionServeHTTP(t *testing.T) {
	tests := []struct {
		name     string
		setPrime func()
		cleanup  func()
		want     string
	}{
		{
			name:     "unmodified",
			setPrime: func() {},
			cleanup:  func() {},
			want:     `{"Version":"dev","GitCommit":"HEAD","RancherPrime":"false"}`,
		},
		{
			name:     "prime=true",
			setPrime: func() { os.Setenv("RANCHER_VERSION_TYPE", "prime") },
			cleanup:  func() { os.Unsetenv("RANCHER_VERSION_TYPE") },
			want:     `{"Version":"dev","GitCommit":"HEAD","RancherPrime":"true"}`,
		},
		{
			name:     "prime=false",
			setPrime: func() { os.Setenv("RANCHER_VERSION_TYPE", "false") },
			cleanup:  func() { os.Unsetenv("RANCHER_VERSION_TYPE") },
			want:     `{"Version":"dev","GitCommit":"HEAD","RancherPrime":"false"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setPrime()
			defer tt.cleanup()
			req := httptest.NewRequest(http.MethodGet, "/rancherversion", nil)
			rr := httptest.NewRecorder()
			handler := NewVersionHandler()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			resp := rr.Result()
			body, err := io.ReadAll(resp.Body)
			assert.Nil(t, err)
			assert.Equal(t, tt.want, string(body))
		})
	}
}
