package management

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func TestGetBootstrapPassword(t *testing.T) {
	tests := []struct {
		name      string
		secret    *corev1.Secret
		expected  string
		generated bool
		expectErr bool
	}{
		{
			name: "secret exists and has key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cattleNamespace,
					Name:      bootstrapPasswordSecretName,
				},
				Data: map[string][]byte{
					bootstrapPasswordSecretKey: []byte("test"),
				},
			},
			expected: "test",
		},
		{
			name: "secret exists with no key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       cattleNamespace,
					Name:            bootstrapPasswordSecretName,
					ResourceVersion: "1",
				},
			},
			generated: true,
		},
		{
			name:      "secret does not exist",
			generated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient := fakeclient.NewSimpleClientset(tt.secret)

			password, generated, err := GetBootstrapPassword(context.TODO(), fakeclient.CoreV1().Secrets(cattleNamespace))

			if tt.expectErr {
				assert.NotNil(t, err)
				return
			}

			assert.Nil(t, err)

			assert.Equal(t, tt.generated, generated)

			if !generated {
				assert.Equal(t, tt.expected, password)
			} else if tt.secret != nil {
				secret, err := fakeclient.CoreV1().Secrets(cattleNamespace).Get(context.TODO(), tt.secret.Name, metav1.GetOptions{})
				assert.Nil(t, err)
				assert.Equal(t, password, secret.StringData[bootstrapPasswordSecretKey])
			}
		})
	}
}
