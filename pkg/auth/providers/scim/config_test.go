package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetServiceProviderConfig(t *testing.T) {
	server := &SCIMServer{}

	t.Run("returns valid service provider config", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/ServiceProviderConfig", nil)

		server.GetServiceProviderConfig(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify schemas
		schemas, ok := response["schemas"].([]any)
		require.True(t, ok, "schemas should be present and be an array")
		assert.Equal(t, 1, len(schemas))
		assert.Equal(t, "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig", schemas[0])

		// Verify documentationUri
		assert.Equal(t, "https://ranchermanager.docs.rancher.com", response["documentationUri"])

		// Verify patch support
		patch, ok := response["patch"].(map[string]any)
		require.True(t, ok, "patch should be present and be a map")
		assert.Equal(t, true, patch["supported"])

		// Verify bulk operations (not supported)
		bulk, ok := response["bulk"].(map[string]any)
		require.True(t, ok, "bulk should be present and be a map")
		assert.Equal(t, false, bulk["supported"])
		assert.Equal(t, float64(1000), bulk["maxOperations"]) // JSON numbers are float64
		assert.Equal(t, float64(1048576), bulk["maxPayloadSize"])

		// Verify filter support
		filter, ok := response["filter"].(map[string]any)
		require.True(t, ok, "filter should be present and be a map")
		assert.Equal(t, true, filter["supported"])
		assert.Equal(t, float64(maxPageSize), filter["maxResults"])

		// Verify password change (not supported)
		changePassword, ok := response["changePassword"].(map[string]any)
		require.True(t, ok, "changePassword should be present and be a map")
		assert.Equal(t, false, changePassword["supported"])

		// Verify sort (not supported)
		sort, ok := response["sort"].(map[string]any)
		require.True(t, ok, "sort should be present and be a map")
		assert.Equal(t, false, sort["supported"])

		// Verify etag (not supported)
		etag, ok := response["etag"].(map[string]any)
		require.True(t, ok, "etag should be present and be a map")
		assert.Equal(t, false, etag["supported"])

		// Verify authentication schemes
		authSchemes, ok := response["authenticationSchemes"].([]any)
		require.True(t, ok, "authenticationSchemes should be present and be an array")
		assert.Equal(t, 1, len(authSchemes))

		scheme, ok := authSchemes[0].(map[string]any)
		require.True(t, ok, "authentication scheme should be a map")
		assert.Equal(t, "oauthbearertoken", scheme["type"])
		assert.Equal(t, "OAuth Bearer Token", scheme["name"])
		assert.Equal(t, "Authentication scheme using the OAuth Bearer Token", scheme["description"])
		assert.Equal(t, "http://tools.ietf.org/html/draft-ietf-scim-core-protocol-10#section-3.1", scheme["specUri"])
		assert.Equal(t, true, scheme["primary"])
	})
}
