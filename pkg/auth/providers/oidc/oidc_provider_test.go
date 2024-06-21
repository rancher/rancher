package oidc

import (
	"encoding/base64"
	"fmt"
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

func TestParseACRFromAccessToken(t *testing.T) {
	header := base64.URLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	validClaims := base64.URLEncoding.EncodeToString([]byte(`{"acr":"example_acr"}`))
	invalidBase64Claims := "invalid_base64_claims"
	noAcrClaims := base64.URLEncoding.EncodeToString([]byte(`{"sub":"1234567890"}`))

	tests := []struct {
		name        string
		token       string
		expectedACR string
		wantError   bool
	}{
		{
			name:        "valid token with ACR",
			token:       fmt.Sprintf("%s.%s.", header, validClaims),
			expectedACR: "example_acr",
		},
		{
			name:        "invalid token format",
			token:       "invalid.token",
			expectedACR: "",
			wantError:   true,
		},
		{
			name:        "invalid base64 decoding",
			token:       fmt.Sprintf("%s.%s.", header, invalidBase64Claims),
			expectedACR: "",
			wantError:   true,
		},
		{
			name:        "valid token without ACR claim",
			token:       fmt.Sprintf("%s.%s.", header, noAcrClaims),
			expectedACR: "",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acr, err := parseACRFromAccessToken(tt.token)
			if acr != tt.expectedACR {
				t.Fatalf("expected acr to be '%s', got '%s'", tt.expectedACR, acr)
			}
			if (err != nil) != tt.wantError {
				t.Fatalf("expected error: %v, got error: %v", tt.wantError, err)
			}
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
