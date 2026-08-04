package configserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// newTestConfigServer wires enough of an RKE2ConfigServer for findMachineByClusterToken:
//   - secretsCache: returns tokenSecret by tokenIndex, readySecret by (namespace, name).
//   - secrets: allows Delete of the MachineRequest secret at the end of findMachineByClusterToken.
//   - mgmtClusterCache: mgmt cluster is present when the namespace looks like a management
//     namespace (c-xxxxx or local) — no turtles labels / administrated annotation, so the
//     resolver returns KindImported. Otherwise NotFound → resolver returns KindV2Prov.
func newTestConfigServer(t *testing.T, tokenSecret *corev1.Secret, readySecret *corev1.Secret, mgmtCluster *apimgmtv3.Cluster) *RKE2ConfigServer {
	ctrl := gomock.NewController(t)

	secretsCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretsCache.EXPECT().GetByIndex(crtTokenIndex, "tok").Return([]*corev1.Secret{tokenSecret}, nil).AnyTimes()
	secretsCache.EXPECT().Get(readySecret.Namespace, readySecret.Name).Return(readySecret, nil).AnyTimes()

	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mgmtClusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	if mgmtCluster != nil {
		mgmtClusterCache.EXPECT().Get(mgmtCluster.Name).Return(mgmtCluster, nil).AnyTimes()
	}
	// Any other lookup — including the KindV2Prov "fleet-default" case — returns NotFound so the
	// resolver falls through to KindV2Prov.
	mgmtClusterCache.EXPECT().Get(gomock.Any()).Return(nil, apierrors.NewNotFound(
		schema.GroupResource{Group: "management.cattle.io", Resource: "clusters"}, "")).AnyTimes()

	return &RKE2ConfigServer{
		secretsCache:     secretsCache,
		secrets:          secrets,
		mgmtClusterCache: mgmtClusterCache,
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
		name       string
		namespace  string
		wantKind   CallerKind
		wantRefAPI string
		wantRefKnd string
	}{
		{
			name:       "management namespace is treated as imported",
			namespace:  "c-abcde",
			wantKind:   KindImported,
			wantRefAPI: "management.cattle.io/v3",
			wantRefKnd: "Node",
		},
		{
			name:       "non-management namespace is treated as custom",
			namespace:  "fleet-default",
			wantKind:   KindV2Prov,
			wantRefAPI: "cluster.x-k8s.io/v1beta2",
			wantRefKnd: "Machine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: tt.namespace, Name: "crt-token-system"},
			}
			secretName := machineRequestSecretName(tt.wantKind, "machine-1")
			readySecret := readyMachineSecret(tt.namespace, secretName)

			// Seed a matching mgmt cluster only for the imported case. The KindV2Prov branch
			// relies on mgmt cluster lookup returning NotFound (fleet-default has no matching
			// v3 cluster), which newTestConfigServer covers via a catch-all NotFound mock.
			var mgmtCluster *apimgmtv3.Cluster
			if tt.wantKind == KindImported {
				mgmtCluster = &apimgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: tt.namespace},
				}
			}

			r := newTestConfigServer(t, tokenSecret, readySecret, mgmtCluster)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer tok")
			req.Header.Set(machineIDHeader, "machine-1")

			ref, err := r.findMachineByClusterToken(req)
			require.NoError(t, err)
			require.NotNil(t, ref)
			assert.Equal(t, tt.wantRefAPI, ref.APIVersion)
			assert.Equal(t, tt.wantRefKnd, ref.Kind)
			assert.Equal(t, "fleet-default", ref.Namespace)
			assert.Equal(t, "machine-1", ref.Name)
		})
	}
}
