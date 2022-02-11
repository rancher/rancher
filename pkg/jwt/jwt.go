package jwt

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/rancher/dynamiclistener/cert"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"gopkg.in/square/go-jose.v2"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	JWKeySecretName        = "cattle-jwt-key"
	rsaKeySize             = 2048
	rsaPrivateKeyBlockType = "RSA PRIVATE KEY"
	jwksContentType        = "application/jwk-set+json"
	headerCacheControl     = "Cache-Control"
	headerContentType      = "Content-Type"
	cacheControl           = "public, max-age=3600"
)

type Signer struct {
	privateKey *rsa.PrivateKey
	jwksBytes  []byte
}

func (s *Signer) ServeJWKS(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set(headerContentType, jwksContentType)
	resp.Header().Set(headerCacheControl, cacheControl)
	resp.Write(s.jwksBytes)
}

func (s *Signer) SignUser(info user.Info, audience string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud":    audience,
		"exp":    time.Now().Add(time.Duration(settings.JWTProxyExpirationSeconds.GetInt()) * time.Second).Unix(),
		"iat":    time.Now().Unix(),
		"sub":    info.GetName(),
		"iss":    "Rancher",
		"groups": info.GetGroups(),
		"extra":  info.GetExtra(),
	})
	return token.SignedString(s.privateKey)
}

func (s *Signer) newKey() ([]byte, error) {
	key, err := rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  rsaPrivateKeyBlockType,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), nil
}

func (s *Signer) saveNewKey(ctx context.Context, secrets v1.SecretsGetter) (*corev1.Secret, error) {
	privateKey, err := s.newKey()
	if err != nil {
		return nil, err
	}
	key, err := secrets.Secrets(namespaces.System).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWKeySecretName,
			Namespace: namespaces.System,
		},
		Data: map[string][]byte{
			"key": privateKey,
		},
	}, metav1.CreateOptions{})
	if apierror.IsAlreadyExists(err) {
		return secrets.Secrets(namespaces.System).Get(ctx, JWKeySecretName, metav1.GetOptions{})
	}
	return key, err
}

func (s *Signer) decodeKey(secret *corev1.Secret) error {
	privateKey, err := cert.ParsePrivateKeyPEM(secret.Data["key"])
	if err != nil {
		return err
	}
	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("key found in %s/%s is not a RSA private key", rsaPrivateKey)
	}
	s.privateKey = rsaPrivateKey
	return nil
}

func New(ctx context.Context, secrets v1.SecretsGetter) (*Signer, error) {
	s := &Signer{}
	key, err := secrets.Secrets(namespaces.System).Get(ctx, JWKeySecretName, metav1.GetOptions{})
	if apierror.IsNotFound(err) {
		key, err = s.saveNewKey(ctx, secrets)
	}
	if err != nil {
		return nil, err
	}
	return s, s.decodeKey(key)
}

func (s *Signer) saveJWKS() error {
	keyID, err := keyID(s.privateKey.PublicKey)
	if err != nil {
		return err
	}

	jwkBytes, err := json.Marshal(&jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Algorithm: string(jose.RS256),
				Key:       s.privateKey.PublicKey,
				KeyID:     keyID,
				Use:       "sig",
			},
		},
	})
	s.jwksBytes = jwkBytes
	return err
}

func keyID(publicKey rsa.PublicKey) (string, error) {
	data, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(hash[:]), nil
}
