package provider

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
)

const (
	privateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAqpXFceskscHq4hxKlJtbAvfh0YF3Wcnjy+k1U2ZxbiHaByLr
USuP7+TgmLaonsh63mW0xa0ReC7MgFWBf4z03S5FWZUs4IpFG6BwrQYYCsANwJPD
lUxX42OeB28iZ2J6e/Laai3dv0YkzORlkl8mkIt9LDDbcdnCR+78I3a6PHE5keO7
NRuyNNVZcQ6RQ9F/sQfxzpnGkG0uP1eRwk81Ii1ZrkVRYnNkuYwH+1FF8R5QYea5
T4EN7+co6G3phO6irKAHWkNgX23PUYMSj+qyLcf7v+1+UumE8jELoNNY7F1M63Xb
X0i14qfcodj4H7WQQIj0LU5NkZJUAMmkxOJkWQIDAQABAoIBADWgjgjpLoj/eJMK
59teF7eQLBrMA7RjhsylDRGiBAjmdX+G18mV01NMddssmUgJqK7f9Husk/BfbgTu
XJ63tocOM9kcz5XrghxUTPfoEYjXpbsj+PmnnX2r5JNbucocqHrs9wMoViz6pTkQ
mGnypdINOBW7alGZbr1kgTm46oVzYliAwnsN6noAxKq0rLzLCi8Gr/iJc/5kaHwO
1Lu9+DwAcNDNYzO+Q12k1VaFYRvyTGAP5WjqG2g/hRz6MMCBearFhWmTUr8rbMKG
dgfQUu72ZBwDwHbaJl4EhPHBdLil9eY2popz1IUouYP6WJ7mx0DAevgSVowHg9Ze
4yM4JX0CgYEA7zkpU0V1m5511aRCUvJij6+XXK15eTAp+NQLp4fQ4gFJ1ap+bkP7
AuQjJu5cxuwP+wniMZby5qEWC6x6MWlo19Gj2Rjpfh7zRTJVToVCKDKNsyAmN58G
XgxVvwzwgJVNTLs6tPz6eQJEdSM7QcrPSk6SqpNbsKB/w/BHq4wFQEcCgYEAtoxU
VO14K3zl5j2HbRszCPrUwYXBB8IxwKbvNjo2G9X+G7aQJ0IWh+5VH3vUh8Awgvcf
CkB4VKGYeY+budz+zQ038ceffJsuiM4pnwkTcAibE5M+MYjmeq4a+spanqkDBZde
BMJ5MVIwc6zXFc4d0z1st5vYUDKwrWPJNtzzJl8CgYAPjbLfJCP6YzocEtrxE6tO
1kbMEsdUFsqT0A2V7eGp4BWR28zulGLslDKo5FSJ5m0/kCvxt3PBhPWu+p7TOZxE
c+/oPJNpzM9aT2R2f3mGrYrC+7MgaKl8Ueb9TfURFyP4ei/d9pi+Z2RWDV1b8Li7
hxJIHt6WREkqTyQJxkfNHwKBgBhHGYAiBPVjqv+v9y7fiy4Kjfke3Mk3Xn6MtQu1
OjUBhMYSxaEy/OQfIlsJkP5s5QbF7u7iHB7FTw37t25Eoe6Lb4FMVz2vNcUkBg0M
m/Uub9Fup7rWxjBSr2vjNaIcQDaiJvLjGlMg5yi7N+/Cddz+MlNI+r/PvgeRWdVe
FUhpAoGAHwGjTPNFDN6pc2vIWO64BRQPQhoiW+7+Lp5F9uyQbXWe3XUokJm9GqjO
lBJVOgny3rF/i0kqJgs3xtIKAw+JlTkYlQCLftatkDFmJrx3FJuJkfso/2lGwKfb
p0memqW7VLNS23qAagWt07F06eh+fg3n2CdKmW0EuTVzfpDfkAw=
-----END RSA PRIVATE KEY-----`
	publicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAqpXFceskscHq4hxKlJtb
Avfh0YF3Wcnjy+k1U2ZxbiHaByLrUSuP7+TgmLaonsh63mW0xa0ReC7MgFWBf4z0
3S5FWZUs4IpFG6BwrQYYCsANwJPDlUxX42OeB28iZ2J6e/Laai3dv0YkzORlkl8m
kIt9LDDbcdnCR+78I3a6PHE5keO7NRuyNNVZcQ6RQ9F/sQfxzpnGkG0uP1eRwk81
Ii1ZrkVRYnNkuYwH+1FF8R5QYea5T4EN7+co6G3phO6irKAHWkNgX23PUYMSj+qy
Lcf7v+1+UumE8jELoNNY7F1M63XbX0i14qfcodj4H7WQQIj0LU5NkZJUAMmkxOJk
WQIDAQAB
-----END PUBLIC KEY-----`
	privateKeySmall = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAqpXFceskscHq4hxKlJtbAvfh0YF3Wcnjy+k1U2ZxbiHaByLr
USuP7+TgmLaonsh63mW0xa0ReC7MgFWBf4z03S5FWZUs4IpFG6BwrQYYCsANwJPD
lUxX42OeB28iZ2J6e/Laai3dv0YkzORlkl8mkIt9LDDbcdnCR+78I3a6PHE5keO7
NRuyNNVZcQ6RQ9F/sQfxzpnGkG0uP1eRwk81Ii1ZrkVRYnNkuYwH+1FF8R5QYea5
T4EN7+co6G3phO6irKAHWkNgX23PUYMSj+qyLcf7v+1+UumE8jELoNNY7F1M63Xb
X0i14qfcodj4H7WQQIj0LU5NkZJUAMmkxOJkWQIDAQABAoIBADWgjgjpLoj/eJMK
59teF7eQLBrMA7RjhsylDRGiBAjmdX+G18mV01NMddssmUgJqK7f9Husk/BfbgTu
XJ63tocOM9kcz5XrghxUTPfoEYjXpbsj+PmnnX2r5JNbucocqHrs9wMoViz6pTkQ
mGnypdINOBW7alGZbr1kgTm46oVzYliAwnsN6noAxKq0rLzLCi8Gr/iJc/5kaHwO
1Lu9+DwAcNDNYzO+Q12k1VaFYRvyTGAP5WjqG2g/hRz6MMCBearFhWmTUr8rbMKG
dgfQUu72ZBwDwHbaJl4EhPHBdLil9eY2popz1IUouYP6WJ7mx0DAevgSVowHg9Ze
4yM4JX0CgYEA7zkpU0V1m5511aRCUvJij6+XXK15eTAp+NQLp4fQ4gFJ1ap+bkP7
AuQjJu5cxuwP+wniMZby5qEWC6x6MWlo19Gj2Rjpfh7zRTJVToVCKDKNsyAmN58G
XgxVvwzwgJVNTLs6tPz6eQJEdSM7QcrPSk6SqpNbsKB/w/BHq4wFQEcCgYEAtoxU
VO14K3zl5j2HbRszCPrUwYXBB8IxwKbvNjo2G9X+G7aQJ0IWh+5VH3vUh8Awgvcf
CkB4VKGYeY+budz+zQ038ceffJsuiM4pnwkTcAibE5M+MYjmeq4a+spanqkDBZde
BMJ5MVIwc6zXFc4d0z1st5vYUDKwrWPJNtzzJl8CgYAPjbLfJCP6YzocEtrxE6tO
1kbMEsdUFsqT0A2V7eGp4BWR28zulGLslDKo5FSJ5m0/kCvxt3PBhPWu+p7TOZxE
c+/oPJNpzM9aT2R2f3mGrYrC+7MgaKl8Ueb9TfURFyP4ei/d9pi+Z2RWDV1b8Li7
hxJIHt6WREkqTyQJxkfNHwKBgBhHGYAiBPVjqv+v9y7fiy4Kjfke3Mk3Xn6MtQu1
OjUBhMYSxaEy/OQfIlsJkP5s5QbF7u7iHB7FTw37t25Eoe6Lb4FMVz2vNcUkBg0M
m/Uub9Fup7rWxjBSr2vjNaIcQDaiJvLjGlMg5yi7N+/Cddz+MlNI+r/PvgeRWdVe
FUhpAoGAHwGjTPNFDN6pc2vIWO64BRQPQhoiW+7+Lp5F9uyQbXWe3XUokJm9GqjO
lBJVOgny3rF/i0kqJgs3xtIKAw+JlTkYlQCLftatkDFmJrx3FJuJkfso/2lGwKfb
p0memqW7VLNS23qAagWt07F06eh+fg3n2CdKmW0EuTVzfpDfkAw=
-----END RSA PRIVATE KEY-----`
	publicKeySmall = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDa2fB2m18KR8whoq1DY51UYMcD
N1rcfAIT6pG7RiXsUoEd6YBA+LbOWe4b7cNZnvvz9S17b+r3bZu1NcE2AEfWbDoI
a5saf1YWiw9zRIRqG4BOBDeySbxbRhP15nWaCBy5fCQ3k1T3mloB0vWcg6IzdDMs
sZe92xd428OAH9tLlQIDAQAB
-----END PUBLIC KEY-----`
)

func TestJWKSEndpoint(t *testing.T) {
	ctlr := gomock.NewController(t)

	tests := map[string]struct {
		secretCache  func() corecontrollers.SecretCache
		expectedCode int
		expectedBody string
	}{
		"jwks success response": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(&v1.Secret{
					Data: map[string][]byte{
						"key.pem": []byte(privateKey),
						"key.pub": []byte(publicKey),
					},
				}, nil)

				return mock
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"keys":[{"kty":"RSA","use":"sig","kid":"key","n":"qpXFceskscHq4hxKlJtbAvfh0YF3Wcnjy-k1U2ZxbiHaByLrUSuP7-TgmLaonsh63mW0xa0ReC7MgFWBf4z03S5FWZUs4IpFG6BwrQYYCsANwJPDlUxX42OeB28iZ2J6e_Laai3dv0YkzORlkl8mkIt9LDDbcdnCR-78I3a6PHE5keO7NRuyNNVZcQ6RQ9F_sQfxzpnGkG0uP1eRwk81Ii1ZrkVRYnNkuYwH-1FF8R5QYea5T4EN7-co6G3phO6irKAHWkNgX23PUYMSj-qyLcf7v-1-UumE8jELoNNY7F1M63XbX0i14qfcodj4H7WQQIj0LU5NkZJUAMmkxOJkWQ","e":"AQAB"}]}`, // contains modulus and exponent for the public key
		},
		"jwks can't get secret": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(nil, errors.New("unexpected error"))

				return mock
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"error":"server_error","error_description":"failed to get secret with public keys"}`,
		},
		"jwks ignores 1024 key": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(&v1.Secret{
					Data: map[string][]byte{
						"key.pem": []byte(privateKeySmall),
						"key.pub": []byte(publicKeySmall),
					},
				}, nil)

				return mock
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"keys":null}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			h := jwksHandler{secretCache: test.secretCache()}

			h.jwksEndpoint(rec, &http.Request{})

			assert.Equal(t, test.expectedCode, rec.Code)
			assert.JSONEq(t, test.expectedBody, strings.TrimSpace(rec.Body.String()))
		})
	}
}

func TestGetSigningKey(t *testing.T) {
	ctlr := gomock.NewController(t)
	block, _ := pem.Decode([]byte(privateKey))
	privKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	tests := map[string]struct {
		secretCache func() corecontrollers.SecretCache
		expectedKid string
		expectedKey *rsa.PrivateKey
		expectedErr string
	}{
		"get signing key": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(&v1.Secret{
					Data: map[string][]byte{
						"key.pem": []byte(privateKey),
						"key.pub": []byte(publicKey),
					},
				}, nil)

				return mock
			},
			expectedKid: "key",
			expectedKey: privKey,
		},
		"error retrieving secret": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(nil, errors.New("unexpected error"))

				return mock
			},
			expectedErr: "unexpected error",
		},
		"no signing key in secret": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(&v1.Secret{
					Data: map[string][]byte{
						"key.pub": []byte(publicKey),
					},
				}, nil)

				return mock
			},
			expectedErr: "signing key not found",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := jwksHandler{secretCache: test.secretCache()}

			key, kid, err := h.GetSigningKey()

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expectedKid, kid)
			assert.Equal(t, test.expectedKey, key)
		})
	}
}

func TestGetPublicKey(t *testing.T) {
	ctlr := gomock.NewController(t)
	block, _ := pem.Decode([]byte(privateKey))
	privKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	tests := map[string]struct {
		secretCache func() corecontrollers.SecretCache
		kid         string
		expectedKey *rsa.PublicKey
		expectedErr string
	}{
		"get signing key": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(&v1.Secret{
					Data: map[string][]byte{
						"key.pem": []byte(privateKey),
						"key.pub": []byte(publicKey),
					},
				}, nil)

				return mock
			},
			kid:         "key",
			expectedKey: &privKey.PublicKey,
		},
		"error retrieving secret": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(nil, errors.New("unexpected error"))

				return mock
			},
			expectedErr: "unexpected error",
		},
		"no signing key in secret": {
			secretCache: func() corecontrollers.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(keySecretNamespace, keySecretName).Return(&v1.Secret{
					Data: map[string][]byte{
						"key.pem": []byte(privateKey),
					},
				}, nil)

				return mock
			},
			expectedErr: "public key not found",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cache := test.secretCache()
			h := jwksHandler{secretCache: cache, oidcKeyClient: NewOIDCKeyClient(cache)}

			key, err := h.GetPublicKey(test.kid)

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expectedKey, key)
		})
	}
}
