package configserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestConfigServer(t *testing.T, tokenSecret *corev1.Secret, readySecret *corev1.Secret) *RKE2ConfigServer {
	ctrl := gomock.NewController(t)

	secretsCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretsCache.EXPECT().GetByIndex(crtTokenIndex, "tok").Return([]*corev1.Secret{tokenSecret}, nil).AnyTimes()
	secretsCache.EXPECT().Get(readySecret.Namespace, readySecret.Name).Return(readySecret, nil).AnyTimes()

	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return &RKE2ConfigServer{
		secretsCache: secretsCache,
		secrets:      secrets,
	}
}

func readyMachineSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels: map[string]string{
				capr.MachineNameLabel:      "machine-1",
				capr.MachineNamespaceLabel: "fleet-default",
			},
		},
	}
}

func TestFindMachineByClusterToken(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantKind  string
	}{
		{
			name:      "management namespace is treated as imported",
			namespace: "c-abcde",
			wantKind:  "Node",
		},
		{
			name:      "non-management namespace is treated as custom",
			namespace: "fleet-default",
			wantKind:  "Machine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: tt.namespace, Name: "crt-token-system"},
			}
			secretName := machineRequestSecretName(tt.wantKind == "Node", "machine-1")
			readySecret := readyMachineSecret(tt.namespace, secretName)

			r := newTestConfigServer(t, tokenSecret, readySecret)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer tok")
			req.Header.Set(machineIDHeader, "machine-1")

			ref, err := r.findMachineByClusterToken(req)
			require.NoError(t, err)
			require.NotNil(t, ref)
			assert.Equal(t, tt.wantKind, ref.Kind)
			assert.Equal(t, "fleet-default", ref.Namespace)
			assert.Equal(t, "machine-1", ref.Name)
		})
	}
}
