package saml

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/crewjam/saml"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
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
	provider.getSamlConfig = func(string) (*apiv3.SamlConfig, error) {
		return &apiv3.SamlConfig{
			RancherAPIHost: "https://rancher.example.com",
		}, nil
	}
	provider.serviceProvider = &saml.ServiceProvider{
		MetadataURL: testParseURL(t, "https://rancher.example.com/saml/metadata"),
	}
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

func TestTestAndEnableUsesRequestConfigName(t *testing.T) {
	setupSamlProviderTypes(t, OKTAName)
	provider := samlProviderTypes[OKTAName]
	provider.userMGR = &fakeUserManager{userName: "test-user"}

	var requestedConfig string
	provider.getSamlConfig = func(configName string) (*apiv3.SamlConfig, error) {
		requestedConfig = configName
		return testSamlConfig(t, configName, client.OKTAConfigType), nil
	}

	body := bytes.NewBufferString(`{"finalRedirectUrl":"https://attacker.example.com/login"}`)
	req := httptest.NewRequest(http.MethodPost, "/v3/oktaConfigs/okta-eu?action=testAndEnable", body)
	apiCtx := &types.APIContext{Request: req, Response: httptest.NewRecorder(), ID: "okta-eu"}

	err := provider.testAndEnable(apiCtx)
	assert.ErrorContains(t, err, "Invalid redirect URL 400")

	assert.Equal(t, "okta-eu", requestedConfig)
	oktaEU, ok := getSamlProvider("okta-eu")
	require.True(t, ok, "the requested config should be initialized")
	assert.Equal(t, "/v1-saml/okta-eu/saml/acs", oktaEU.serviceProvider.AcsURL.Path)
	_, ok = getSamlProvider(OKTAName)
	assert.False(t, ok, "the default config should not be initialized")
}

func testParseURL(t *testing.T, urlStr string) url.URL {
	parsedURL, err := url.Parse(urlStr)
	require.NoError(t, err)
	return *parsedURL
}
