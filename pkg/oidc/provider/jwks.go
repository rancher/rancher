package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	oidcerror "github.com/rancher/rancher/pkg/oidc/provider/error"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	keyBits            = 3072
	keySecretNamespace = "cattle-system"
	keySecretName      = "oidc-signing-key"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key Type (e.g., RSA)
	Use string `json:"use"` // Key Usage (e.g., sig)
	Kid string `json:"kid"` // Key ID
	N   string `json:"n"`   // Modulus
	E   string `json:"e"`   // Exponent
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// NewOIDCKeyClient creates and returns a new OIDCKeyClient that can lookup a
// PublicKey by ID.
func NewOIDCKeyClient(secrets corecontrollers.SecretCache) *oidcKeyClient {
	return &oidcKeyClient{secretCache: secrets}
}

type oidcKeyClient struct {
	secretCache corecontrollers.SecretCache
}

// GetPublicKey returns the public key specified by the kid
func (h *oidcKeyClient) GetPublicKey(kid string) (*rsa.PublicKey, error) {
	s, err := h.secretCache.Get(keySecretNamespace, keySecretName)
	if err != nil {
		return nil, err
	}
	for name, value := range s.Data {
		if name == kid+".pub" {
			return getPublicKeyFromSecretData(value)
		}
	}
	return nil, fmt.Errorf("public key not found")
}

type jwksHandler struct {
	*oidcKeyClient
	secretCache  corecontrollers.SecretCache
	secretClient corecontrollers.SecretClient
}

// newJWKSHandler returns a jwks handler. Creates a default signing key.
func newJWKSHandler(secretCache corecontrollers.SecretCache, secretClient corecontrollers.SecretClient) (*jwksHandler, error) {
	_, err := secretClient.Get(keySecretNamespace, keySecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if errors.IsNotFound(err) {
		logrus.Infof("[OIDC provider] creating a new signing key")
		// generate a default RSA private key
		privateKey, err := rsa.GenerateKey(rand.Reader, keyBits)
		if err != nil {
			return nil, err
		}

		// Encode private key to PEM
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})

		// Encode public key to PEM
		publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal public key: %w", err)
		}
		publicKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyDER,
		})

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      keySecretName,
				Namespace: keySecretNamespace,
			},
			Data: map[string][]byte{
				"key.pem": privateKeyPEM,
				"key.pub": publicKeyPEM,
			},
		}

		_, err = secretClient.Create(secret)
		if err != nil {
			return nil, err
		}
	}

	return &jwksHandler{
		oidcKeyClient: NewOIDCKeyClient(secretCache),
		secretCache:   secretCache,
		secretClient:  secretClient,
	}, nil
}

// jwksEndpoint writes the content of the jwks endpoint.
// A default key pair for signing the jwt tokens will be created by Rancher. Additionally, administrators should have the ability to change keys. Keys are stored in a k8s secret called oidc-signing-key in the cattle-system namespace. Only one key will be used for signing,
// but multiple keys can be returned in the jwks endpoint in order to avoid disruption when doing a key rotation. Example:
//
// apiVersion: v1
// kind: Secret
// metadata:
//
//	name: oidc-signing-key
//
// type: Opaque
// data:
//
//	key2.pem: <base64-encoded-private-key>
//	key1.pub: <base64-encoded-public-key>
//	key2.pub: <base64-encoded-public-key>
//
// It will sign jwt tokens with key2.pem, but jwks will return key1.pub and key2.pub in order to avoid disruptions when doing a key rotation from key1 to key2.
// Only one private key (.pem) can be in this secret. Note that the private and public keys must have the same name (kid) with different suffix (.pem and .pub).
func (h *jwksHandler) jwksEndpoint(w http.ResponseWriter, r *http.Request) {
	s, err := h.secretCache.Get(keySecretNamespace, keySecretName)
	if err != nil {
		logrus.Errorf("[OIDC provider] failed to get secret with public keys %v", err)
		oidcerror.WriteError(oidcerror.ServerError, "failed to get secret with public keys", http.StatusInternalServerError, w)
		return
	}
	var keys []JWK
	for name, value := range s.Data {
		if !strings.HasSuffix(name, ".pub") {
			continue
		}

		pubKey, err := getPublicKeyFromSecretData(value)
		if err != nil {
			logrus.Errorf("[OIDC provider] failed to extract public key from secret data %v", err)
			oidcerror.WriteError(oidcerror.ServerError, "failed to extract public key from secret data", http.StatusInternalServerError, w)
			return
		}
		if pubKey.N.BitLen() < 2048 {
			logrus.Warnf("[OIDC provider] ignoring key because the size is less than 2048 bits")
			continue
		}

		keys = append(keys, JWK{
			Kty: "RSA",
			Use: "sig",
			Kid: strings.TrimSuffix(name, ".pub"),
			N:   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(JWKS{Keys: keys}); err != nil {
		oidcerror.WriteError(oidcerror.ServerError, "failed to encode JWKS", http.StatusInternalServerError, w)
	}
}

// GetSigningKey returns the key used for signing jwt tokens, and it's key id (kid)
func (h *jwksHandler) GetSigningKey() (*rsa.PrivateKey, string, error) {
	s, err := h.secretCache.Get(keySecretNamespace, keySecretName)
	if err != nil {
		return nil, "", err
	}
	for name, value := range s.Data {
		if strings.HasSuffix(name, ".pem") {
			return getPrivateKeyFromSecretData(name, value)
		}
	}
	return nil, "", fmt.Errorf("signing key not found")
}

func getPrivateKeyFromSecretData(name string, privateKeyPEM []byte) (*rsa.PrivateKey, string, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, "", fmt.Errorf("failed to decode PEM block")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse RSA private key: %w", err)
	}
	return privateKey, strings.TrimSuffix(name, ".pem"), nil
}

func getPublicKeyFromSecretData(publicKeyPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}
	publicKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return publicKey, nil
}
