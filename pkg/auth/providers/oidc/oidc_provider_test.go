package oidc

import (
	"testing"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

func Test_validateACR(t *testing.T) {
	tests := []struct {
		name          string
		claimACR      string
		configuredACR string
		want          bool
	}{
		{
			name:          "acr set in config and matches token",
			configuredACR: "level1",
			claimACR:      "level1",
			want:          true,
		},
		{
			name:          "acr set in config and does not match token",
			configuredACR: "level1",
			claimACR:      "",
			want:          false,
		},
		{
			name:     "acr not set in config",
			claimACR: "",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isValidACR(tt.claimACR, tt.configuredACR), "isValidACR(%v, %v)", tt.claimACR, tt.configuredACR)
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
