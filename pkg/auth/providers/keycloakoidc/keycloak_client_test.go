package keycloakoidc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeyCloakClientSearchEscapesSearchTerm verifies that the search term is
// percent-encoded before it is sent to Keycloak. A term containing characters
// that are not valid in a URL (for example non-ASCII characters) previously
// produced a malformed request line that Keycloak rejected with an empty body,
// which surfaced to the user as "unexpected end of JSON input".
func TestKeyCloakClientSearchEscapesSearchTerm(t *testing.T) {
	const searchTerm = "研发团队"

	tests := []struct {
		name string
		run  func(t *testing.T, c *KeyCloakClient, issuer string)
	}{
		{
			name: "group search",
			run: func(t *testing.T, c *KeyCloakClient, issuer string) {
				_, err := c.groupSearch(searchTerm, issuer)
				require.NoError(t, err)
			},
		},
		{
			name: "user search",
			run: func(t *testing.T, c *KeyCloakClient, issuer string) {
				_, err := c.searchPrincipals(searchTerm, UserType, &apiv3.OIDCConfig{Issuer: issuer})
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			}))
			t.Cleanup(srv.Close)

			// The admin URL is derived from the issuer, so use an issuer that
			// resolves back to the test server.
			issuer := srv.URL + "/realms/testing"
			c := &KeyCloakClient{httpClient: srv.Client()}

			tt.run(t, c, issuer)

			// The request must carry the percent-encoded term, not the raw
			// characters.
			assert.Equal(t, "search="+url.QueryEscape(searchTerm), rawQuery)
			// ...and it must still decode back to the original term.
			values, err := url.ParseQuery(rawQuery)
			require.NoError(t, err)
			assert.Equal(t, searchTerm, values.Get("search"))
		})
	}
}
