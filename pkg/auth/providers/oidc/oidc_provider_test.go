package oidc

import (
	"testing"

	"github.com/golang-jwt/jwt"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func Test_validateACR(t *testing.T) {
	tests := []struct {
		name        string
		oauth2Token *oauth2.Token
		config      *v32.OIDCConfig
		want        bool
	}{
		{
			name: "acr set in config and matches token",
			config: &v32.OIDCConfig{
				AcrValue: "level1",
			},
			oauth2Token: &oauth2.Token{
				AccessToken: generateAccessToken("level1"),
			},
			want: true,
		},
		{
			name: "acr set in config and does not match token",
			config: &v32.OIDCConfig{
				AcrValue: "level1",
			},
			oauth2Token: &oauth2.Token{
				AccessToken: generateAccessToken(""),
			},
			want: false,
		},
		{
			name:   "acr not set in config",
			config: &v32.OIDCConfig{},
			oauth2Token: &oauth2.Token{
				AccessToken: generateAccessToken(""),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, validateACR(tt.oauth2Token, tt.config), "validateACR(%v, %v)", tt.oauth2Token, tt.config)
		})
	}
}

// generateAccessToken generates an access token with the specified acr.
func generateAccessToken(acr string) string {
	claims := jwt.MapClaims{
		"acr": acr,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test_secret_key"))
	return tokenString
}
