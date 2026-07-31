package saml

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestAndEnableInvalidFinalRedirectURL(t *testing.T) {
	providerName := "okta"
	invalidRedirect := "https://attacker.example.com/login"

	provider := &Provider{
		name:        providerName,
		userMGR:     &fakeUserManager{userName: "test-user"},
		clientState: &fakeClientState{},
	}
	provider.getSamlConfig = func() (*apiv3.SamlConfig, error) {
		return &apiv3.SamlConfig{
			RancherAPIHost: "https://rancher.example.com",
		}, nil
	}
	provider.metadataURL = testParseURL(t, "https://rancher.example.com/saml/metadata")
	originalProvider := SamlProviders[providerName]
	SamlProviders[providerName] = provider
	t.Cleanup(func() {
		SamlProviders[providerName] = originalProvider
	})

	body := bytes.NewBufferString(`{"finalRedirectUrl":"` + invalidRedirect + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1-saml/"+providerName+"/testAndEnable", body)
	res := httptest.NewRecorder()
	apiCtx := &types.APIContext{Request: req, Response: res}

	err := provider.testAndEnable(apiCtx)

	assert.ErrorContains(t, err, "Invalid redirect URL 400: failed to validate final redirection URL")
}

func testParseURL(t *testing.T, urlStr string) url.URL {
	parsedURL, err := url.Parse(urlStr)
	require.NoError(t, err)
	return *parsedURL
}
