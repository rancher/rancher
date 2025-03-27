package provider

import (
	"encoding/json"
	"net/http"

	"github.com/rancher/rancher/pkg/settings"
)

// OpenIDConfiguration represents response from the /.well-known/openid-configuration endpoint
type OpenIDConfiguration struct {
	// Issuer is the OIDC provider url
	Issuer string `json:"issuer"`
	// AuthorizationEndpoint is the authorization endpoint
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	// TokenEndpoint is the token endpoint
	TokenEndpoint string `json:"token_endpoint"`
	// UserInfoEndpoint is the userinfo endpoint
	UserInfoEndpoint string `json:"userinfo_endpoint"`
	// JWKSURI is the jwksuri endpoint
	JWKSURI string `json:"jwks_uri"`
	// ResponseTypesSupported response types supported, only 'code' is supported
	ResponseTypesSupported []string `json:"response_types_supported"`
	// SubjectTypesSupported subject types supported, only 'public' is supported
	SubjectTypesSupported []string `json:"subject_types_supported"`
	// IDTokenSigningAlgsValuesSupported only RS256 is supported
	IDTokenSigningAlgsValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	// CodeChallengeMethodsSupported only S256 is supported
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	// ScopesSupported can be openid, profile, offline_token
	ScopesSupported []string `json:"scopes_supported"`
	// GrantTypesSupported only code is supported
	GrantTypesSupported []string `json:"grant_types_supported"`
}

func openIDConfigurationEndpoint(w http.ResponseWriter, r *http.Request) {
	config := OpenIDConfiguration{
		Issuer:                            oidcProviderHost(),
		AuthorizationEndpoint:             oidcProviderHost() + "/authorize",
		TokenEndpoint:                     oidcProviderHost() + "/token",
		JWKSURI:                           oidcProviderHost() + "/.well-known/jwks.json",
		UserInfoEndpoint:                  oidcProviderHost() + "/userinfo",
		ResponseTypesSupported:            []string{"code"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgsValuesSupported: []string{"RS256"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		ScopesSupported:                   []string{"openid", "profile", "offline_access"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&config); err != nil {
		http.Error(w, "failed to encode JWKS", http.StatusInternalServerError)
	}
}

func oidcProviderHost() string {
	return settings.ServerURL.Get() + "/oidc"
}
