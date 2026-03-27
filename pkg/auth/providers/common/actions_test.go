package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigNameFromRequest(t *testing.T) {
	tests := []struct {
		path       string
		name       string
		body       string
		wantName   string
		wantErrMsg string
	}{
		{
			path:     "/v3/githubConfigs/github",
			name:     "valid configName",
			body:     `{"configName":"myProvider"}`,
			wantName: "myProvider",
		},
		{
			path:     "/v3/githubConfigs/github",
			name:     "extra fields ignored",
			body:     `{"configName":"myProvider","other":"value"}`,
			wantName: "myProvider",
		},
		{
			path:     "/v3/githubConfigs/github",
			name:     "empty configName",
			body:     `{"configName":""}`,
			wantName: "github",
		},
		{
			path:     "/v3/githubConfigs/github",
			name:     "missing configName field",
			body:     `{"other":"value"}`,
			wantName: "github",
		},
		{
			path:       "/v3/githubConfigs/github",
			name:       "invalid JSON",
			body:       `not-json`,
			wantErrMsg: "invalid character",
		},
		{
			path:     "/v3/githubConfigs/github-2",
			name:     "empty body",
			body:     ``,
			wantName: "github-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configNameFromRequest(newAPIContext(tt.path, tt.body))
			if tt.wantErrMsg != "" {
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got)
		})
	}
}

func newAPIContext(path, body string) *types.APIContext {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	return &types.APIContext{Request: req}
}
