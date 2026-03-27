package publicapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v3public "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	"github.com/stretchr/testify/require"
)

func TestListAuthProviderTypes(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1-public/authprovider-types", nil)

	listAuthProviderTypes(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var got map[string]authProvider
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, authProviderTypes, got)
}

func TestAuthProviderTypesLDAPProviders(t *testing.T) {
	for _, providerType := range []string{
		v3public.ActiveDirectoryProviderType,
		v3public.OpenLdapProviderType,
		v3public.FreeIpaProviderType,
	} {
		require.Equal(t, "ldap", authProviderTypes[providerType].Type, providerType)
	}
}
