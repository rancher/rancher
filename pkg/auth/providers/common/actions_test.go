package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAPIContext(body string) *types.APIContext {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	return &types.APIContext{Request: req}
}

func TestConfigNameFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantName   string
		wantErrMsg string
	}{
		{
			name:     "valid configName",
			body:     `{"configName":"myProvider"}`,
			wantName: "myProvider",
		},
		{
			name:     "extra fields ignored",
			body:     `{"configName":"myProvider","other":"value"}`,
			wantName: "myProvider",
		},
		{
			name:     "empty configName",
			body:     `{"configName":""}`,
			wantName: "",
		},
		{
			name:     "missing configName field",
			body:     `{"other":"value"}`,
			wantName: "",
		},
		{
			name:       "invalid JSON",
			body:       `not-json`,
			wantErrMsg: "invalid character",
		},
		{
			name:       "empty body",
			body:       ``,
			wantErrMsg: io.EOF.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configNameFromRequest(newAPIContext(tt.body))
			if tt.wantErrMsg != "" {
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got)
		})
	}
}
