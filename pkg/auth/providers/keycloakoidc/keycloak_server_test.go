package keycloakoidc

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// newFakeKeycloakServer creates an http server that responds like a Keycloak
// server.
func newFakeKeycloakServer(t *testing.T, privateKey *rsa.PrivateKey, verifyFunc func(t *testing.T, r *http.Request) bool) *httptest.Server {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	resp := newOIDCResponses(t, privateKey, server.Listener.Addr().String())

	mux.HandleFunc("/realms/testing/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.config)
	})
	mux.HandleFunc("/realms/testing/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.jwks)
	})
	mux.HandleFunc("/realms/testing/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp.user))
	})
	mux.HandleFunc("/realms/testing/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.token)
	})

	mux.HandleFunc("/admin/realms/testing/users", func(w http.ResponseWriter, r *http.Request) {
		if !verifyFunc(t, r) {
			http.Error(w, "Failed to verify request", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.searchResults)
	})

	return server
}

type fakeOIDCResponses struct {
	user          string
	config        providerJSON
	jwks          jsonWebKeySet
	token         *Token
	searchResults any
}

type Token struct {
	oauth2.Token
	IDToken string `json:"id_token"`
}

func newOIDCResponses(t *testing.T, privateKey *rsa.PrivateKey, addr string) fakeOIDCResponses {
	jwtToken := jwt.New(jwt.SigningMethodRS256)
	jwtToken.Claims = jwt.RegisteredClaims{
		Audience:  []string{"test"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		Issuer:    "http://" + addr,
	}
	jwtSrt, err := jwtToken.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign fake JWT: %v", err)
	}
	// token returned from the /token endpoint
	token := &Token{
		Token: oauth2.Token{
			AccessToken:  jwtSrt,
			Expiry:       time.Now().Add(5 * time.Minute), // expires in the future
			RefreshToken: jwtSrt,
		},
		IDToken: jwtSrt,
	}

	realmURL := "http://" + addr + "/realms/testing"
	return fakeOIDCResponses{
		user: `{
				"sub": "a8d0d2c4-6543-4546-8f1a-73e1d7dffcbd",
				"email_verified": true,
				"groups": [
					"admingroup"
				],
				"full_group_path": [
					"/admingroup"
				],
				"roles": [
					"adminrole"
				]
      }`,
		config: providerJSON{
			Issuer:      realmURL,
			UserInfoURL: realmURL + "/user",
			JWKSURL:     realmURL + "/.well-known/jwks.json",
			AuthURL:     realmURL + "/auth",
			TokenURL:    realmURL + "/token",
		},
		token: token,
		jwks: jsonWebKeySet{
			Keys: []jsonWebKey{
				{
					Kty: "RSA",
					Kid: "example-key-id",
					Use: "sig",
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(bigIntToBytes(privateKey.PublicKey.E)),
				},
			},
		},
		searchResults: []map[string]any{
			{
				"id":        "9f3f3bab-1c7f-4f1e-970c-6bd2db77684b",
				"email":     "user@example.com",
				"username":  "testing",
				"enabled":   true,
				"firstName": "Testing",
				"lastName":  "User",
			},
		},
	}
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// Helper function to convert an integer exponent to its minimal big-endian byte representation.
func bigIntToBytes(i int) []byte {
	return big.NewInt(int64(i)).Bytes()
}

type providerJSON struct {
	Issuer        string   `json:"issuer"`
	AuthURL       string   `json:"authorization_endpoint"`
	TokenURL      string   `json:"token_endpoint"`
	DeviceAuthURL string   `json:"device_authorization_endpoint"`
	JWKSURL       string   `json:"jwks_uri"`
	UserInfoURL   string   `json:"userinfo_endpoint"`
	Algorithms    []string `json:"id_token_signing_alg_values_supported"`
}
