package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCSRF_IssuedCookieHasSameSiteLax(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.com/v3/things", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; test)")
	rec := httptest.NewRecorder()
	ctx := &types.APIContext{Method: http.MethodGet, Request: req, Response: rec}

	require.NoError(t, api.CheckCSRF(ctx))

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	assert.False(t, cookies[0].HttpOnly)
}
