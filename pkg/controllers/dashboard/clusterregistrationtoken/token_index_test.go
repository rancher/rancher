package clusterregistrationtoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func tokenSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-abcde", Name: name},
		Type:       corev1.SecretTypeOpaque,
		Data:       data,
	}
}

func TestSecretTokenIndexValues(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   []string
	}{
		{
			name:   "nil secret",
			secret: nil,
			want:   nil,
		},
		{
			name:   "not a token secret",
			secret: tokenSecret("some-other-secret", map[string][]byte{tokenDataKey: []byte("abc")}),
			want:   nil,
		},
		{
			name:   "token secret without token data",
			secret: tokenSecret(SecretName("crt1"), map[string][]byte{"expiresAt": []byte("later")}),
			want:   nil,
		},
		{
			name:   "token secret with empty token",
			secret: tokenSecret(SecretName("crt1"), map[string][]byte{tokenDataKey: []byte("")}),
			want:   nil,
		},
		{
			name:   "current token only",
			secret: tokenSecret(SecretName("crt1"), map[string][]byte{tokenDataKey: []byte("current-token")}),
			want:   []string{"current-token"},
		},
		{
			name: "current and previous token (grace period)",
			secret: tokenSecret(SecretName("crt1"), map[string][]byte{
				tokenDataKey:         []byte("current-token"),
				previousTokenDataKey: []byte("previous-token"),
			}),
			want: []string{"current-token", "previous-token"},
		},
		{
			name: "empty previous token is ignored",
			secret: tokenSecret(SecretName("crt1"), map[string][]byte{
				tokenDataKey:         []byte("current-token"),
				previousTokenDataKey: []byte(""),
			}),
			want: []string{"current-token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SecretTokenIndexValues(tt.secret))
		})
	}
}
