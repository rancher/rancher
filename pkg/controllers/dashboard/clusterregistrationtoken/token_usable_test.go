package clusterregistrationtoken

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type secretGetterFunc func(namespace, name string) (*corev1.Secret, error)

func (f secretGetterFunc) Get(namespace, name string) (*corev1.Secret, error) {
	return f(namespace, name)
}

func TestGetTokenFromSecret(t *testing.T) {
	migrated := &v3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Name: "default-token", Namespace: "c-m-test"},
		Status:     v3.ClusterRegistrationTokenStatus{TokenSecretName: "crt-token-default-token"},
	}
	notMigrated := &v3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Name: "default-token", Namespace: "c-m-test"},
		Status:     v3.ClusterRegistrationTokenStatus{Token: "legacy"},
	}
	secretWith := func(data map[string][]byte) secretGetterFunc {
		return func(namespace, name string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: data}, nil
		}
	}

	tests := []struct {
		name         string
		crt          *v3.ClusterRegistrationToken
		secrets      secretGetterFunc
		wantToken    string
		wantNotReady bool
		wantErr      bool
	}{
		{name: "token in secret", crt: migrated, secrets: secretWith(map[string][]byte{tokenDataKey: []byte("abc")}), wantToken: "abc"},
		{name: "nil crt", crt: nil, wantNotReady: true},
		{name: "no token secret name yet", crt: notMigrated, wantNotReady: true},
		{name: "empty token in secret", crt: migrated, secrets: secretWith(map[string][]byte{tokenDataKey: {}}), wantNotReady: true},
		{name: "missing token key", crt: migrated, secrets: secretWith(map[string][]byte{}), wantErr: true},
		{
			name: "secret not in cache",
			crt:  migrated,
			secrets: func(namespace, name string) (*corev1.Secret, error) {
				return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GetTokenFromSecret(tt.secrets, tt.crt)
			switch {
			case tt.wantNotReady:
				require.ErrorIs(t, err, ErrTokenNotReady)
			case tt.wantErr:
				require.Error(t, err)
				assert.NotErrorIs(t, err, ErrTokenNotReady)
			default:
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
