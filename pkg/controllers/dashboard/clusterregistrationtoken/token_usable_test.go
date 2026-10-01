package clusterregistrationtoken

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestTokenSecretUsable(t *testing.T) {
	now := metav1.Now()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "crt-token-default-token", Namespace: "c-m-test"}}

	tests := []struct {
		name       string
		namespace  *corev1.Namespace
		err        error
		wantUsable bool
		wantErr    bool
	}{
		{
			name:       "active namespace",
			namespace:  &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
			wantUsable: true,
		},
		{
			name:      "namespace being deleted",
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", DeletionTimestamp: &now}},
		},
		{
			name:      "namespace terminating",
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating}},
		},
		{
			// The secret can't outlive its namespace, so this is only a cache that's behind.
			name:       "namespace not found",
			err:        apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "c-m-test"),
			wantUsable: true,
		},
		{
			name:    "lookup failure",
			err:     errors.New("boom"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usable, err := TokenSecretUsable(secret, func(name string) (*corev1.Namespace, error) {
				assert.Equal(t, "c-m-test", name)
				return tt.namespace, tt.err
			})
			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, usable, "a token must never be accepted when its namespace can't be checked")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantUsable, usable)
		})
	}
}
