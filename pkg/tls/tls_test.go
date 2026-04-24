package tls

import (
	"testing"

	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// makeInternalSecret builds a tls-rancher-internal secret with the given IPs
// already recorded in dynamiclistener's CN annotations.
func makeInternalSecret(ips ...string) *corev1.Secret {
	annotations := map[string]string{}
	for _, ip := range ips {
		annotations["listener.cattle.io/cn-"+ip] = ip
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "tls-rancher-internal",
			Namespace:   namespace.System,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       []byte("fake-cert"),
			corev1.TLSPrivateKeyKey: []byte("fake-key"),
		},
	}
}

// makeStaticSecret builds a tls-rancher-internal secret marked as static
// (user-provided — must never be deleted).
func makeStaticSecret() *corev1.Secret {
	s := makeInternalSecret()
	s.Annotations[factory.Static] = "true"
	return s
}

func TestEnsureInternalCertSANs(t *testing.T) {
	t.Parallel()

	const clusterIP = "10.43.187.204"

	tests := []struct {
		name         string
		secret       *corev1.Secret // nil means secret does not exist
		clusterIP    string
		expectDelete bool
		expectError  bool
	}{
		{
			name:         "no existing secret — no action needed",
			secret:       nil,
			clusterIP:    clusterIP,
			expectDelete: false,
		},
		{
			name:         "secret already contains clusterIP — no-op",
			secret:       makeInternalSecret(clusterIP),
			clusterIP:    clusterIP,
			expectDelete: false,
		},
		{
			name:         "secret has wrong IP — must delete",
			secret:       makeInternalSecret("10.43.32.193"),
			clusterIP:    clusterIP,
			expectDelete: true,
		},
		{
			name: "secret has no annotations at all — must delete",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-rancher-internal",
					Namespace: namespace.System,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("fake-cert"),
					corev1.TLSPrivateKeyKey: []byte("fake-key"),
				},
			},
			clusterIP:    clusterIP,
			expectDelete: true,
		},
		{
			name:         "static secret (user-provided) — must NOT delete even if IP missing",
			secret:       makeStaticSecret(),
			clusterIP:    clusterIP,
			expectDelete: false,
		},
		{
			name:         "secret has multiple IPs including clusterIP — no-op",
			secret:       makeInternalSecret("10.43.32.193", clusterIP, "172.18.0.2"),
			clusterIP:    clusterIP,
			expectDelete: false,
		},
		{
			name:         "empty clusterIP — no action",
			secret:       makeInternalSecret("10.43.32.193"),
			clusterIP:    "",
			expectDelete: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			secretController := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)

			// Set up Get expectation (only called when clusterIP is non-empty)
			if tt.clusterIP != "" {
				secretController.EXPECT().
					Get(namespace.System, "tls-rancher-internal", gomock.Any()).
					DoAndReturn(func(ns, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
						if tt.secret == nil {
							return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
						}
						return tt.secret, nil
					}).Times(1)
			}

			// Set up Delete expectation
			if tt.expectDelete {
				secretController.EXPECT().
					Delete(namespace.System, "tls-rancher-internal", gomock.Any()).
					Return(nil).
					Times(1)
			}

			err := ensureInternalCertSANs(secretController, tt.clusterIP)
			if tt.expectError && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestEnsureInternalCertSANs_DeleteNotFound verifies that a NotFound error
// on delete (another HA pod already deleted it) is silently ignored.
func TestEnsureInternalCertSANs_DeleteNotFound(t *testing.T) {
	t.Parallel()

	const clusterIP = "10.43.187.204"
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	secretController := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secretController.EXPECT().
		Get(namespace.System, "tls-rancher-internal", gomock.Any()).
		Return(makeInternalSecret("10.43.32.193"), nil).
		Times(1)
	secretController.EXPECT().
		Delete(namespace.System, "tls-rancher-internal", gomock.Any()).
		Return(apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "tls-rancher-internal")).
		Times(1)

	err := ensureInternalCertSANs(secretController, clusterIP)
	if err != nil {
		t.Fatalf("expected nil error on NotFound delete, got: %v", err)
	}
}
