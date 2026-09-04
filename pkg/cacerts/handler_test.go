package cacerts

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func expectedHash(token, nonce string, ca []byte) string {
	digest := hmac.New(sha512.New, []byte(token))
	digest.Write([]byte(nonce))
	digest.Write([]byte{0})
	digest.Write(ca)
	digest.Write([]byte{0})
	return base64.StdEncoding.EncodeToString(digest.Sum(nil))
}

func TestHandler_HMAC(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "crt-token-system"},
		Data: map[string][]byte{
			"token":         []byte("current-tok"),
			"previousToken": []byte("previous-tok"),
		},
	}

	tests := []struct {
		name          string
		authorization string
		wantToken     string
		wantHeader    bool
	}{
		{
			name:          "current token authenticates HMAC with current token",
			authorization: hashToken("current-tok"),
			wantToken:     "current-tok",
			wantHeader:    true,
		},
		{
			name:          "previous token authenticates HMAC with previous token",
			authorization: hashToken("previous-tok"),
			wantToken:     "previous-tok",
			wantHeader:    true,
		},
		{
			name:          "unrecognized hash sets no header",
			authorization: hashToken("wrong-tok"),
			wantHeader:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			mockCache.EXPECT().GetByIndex(tokenHash, tt.authorization).Return([]*corev1.Secret{secret}, nil)

			req := httptest.NewRequest(http.MethodGet, "/cacerts", nil)
			req.Header.Set("Authorization", "Bearer "+tt.authorization)
			req.Header.Set("X-Cattle-Nonce", "nonce")
			rw := httptest.NewRecorder()

			handler(mockCache, rw, req)

			if !tt.wantHeader {
				assert.Empty(t, rw.Header().Get("X-Cattle-Hash"))
				return
			}
			assert.Equal(t, expectedHash(tt.wantToken, "nonce", nil), rw.Header().Get("X-Cattle-Hash"))
		})
	}
}

func TestHandler_NoSecretMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	mockCache.EXPECT().GetByIndex(tokenHash, gomock.Any()).Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/cacerts", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	req.Header.Set("X-Cattle-Nonce", "nonce")
	rw := httptest.NewRecorder()

	handler(mockCache, rw, req)

	assert.Empty(t, rw.Header().Get("X-Cattle-Hash"))
}
