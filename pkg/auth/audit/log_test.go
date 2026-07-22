package audit

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultBodyLimit matches the default value of settings.APIBodyLimit ("1Mi").
const defaultBodyLimit = 1024 * 1024

func TestCopyReqBodyLoginBodyLimit(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		bodySize     int
		keepBody     bool
		wantBodySize int
	}{
		{
			name:         "login endpoint: body under limit is fully preserved",
			url:          "/v3-public/localProviders/local?action=login",
			bodySize:     512,
			keepBody:     false,
			wantBodySize: 512,
		},
		{
			name:         "login endpoint: body over limit is capped to prevent DoS",
			url:          "/v3-public/localProviders/local?action=login",
			bodySize:     defaultBodyLimit + 1,
			keepBody:     false,
			wantBodySize: defaultBodyLimit,
		},
		{
			name:         "non-login endpoint at LevelRequest: large body is not truncated",
			url:          "/v3/clusters",
			bodySize:     defaultBodyLimit + 1,
			keepBody:     true,
			wantBodySize: defaultBodyLimit + 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawBody := bytes.Repeat([]byte("a"), tt.bodySize)
			req := httptest.NewRequest(http.MethodPost, tt.url, bytes.NewReader(rawBody))
			req.Header.Set("Content-Type", "application/json")

			copyReqBody(req, tt.keepBody)

			restored, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Len(t, restored, tt.wantBodySize)
		})
	}
}
