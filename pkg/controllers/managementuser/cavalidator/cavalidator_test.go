// mocks created with the following commands
//
// mockgen --build_flags=--mod=mod -package cavalidator -destination ./v3mgmntMocks_test.go github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3 ClusterInterface,ClusterLister
// mockgen --build_flags=--mod=mod -package cavalidator -destination ./v1coreMocks_test.go github.com/rancher/rancher/pkg/generated/norman/core/v1 SecretController

package cavalidator

import (
	"github.com/golang/mock/gomock"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

type testMocks struct {
	t                 *testing.T
	mockCluster       *MockClusterInterface
	mockClusterLister *MockClusterLister
	mockSecret        *MockSecretController
}

func newMocks(t *testing.T) *testMocks {
	t.Helper()
	ctrl := gomock.NewController(t)
	return &testMocks{
		t:                 t,
		mockCluster:       NewMockClusterInterface(ctrl),
		mockClusterLister: NewMockClusterLister(ctrl),
		mockSecret:        NewMockSecretController(ctrl),
	}
}

func TestCAValidator_clusterConditionManipulation(t *testing.T) {
	type args struct {
		conditionSet           bool
		expectedConditionValue string
		secret                 *corev1.Secret
		cluster                *mgmtv3.Cluster
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "test-empty-secret-unknown",
			args: args{
				conditionSet:           true,
				expectedConditionValue: "Unknown",
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "stv-aggregation",
						Namespace: namespace.System,
					},
				},
				cluster: &mgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "c-cluster",
					},
				},
			},
		},
		{
			name: "test-bad-but-stv-aggregation-secret",
			args: args{
				conditionSet:           true,
				expectedConditionValue: "Unknown",
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "stv-aggregation",
						Namespace: namespace.System,
					},
					Data: map[string][]byte{
						CacertsValid: []byte("asdf"),
						"ca.crt":     []byte("test"),
					},
				},
				cluster: &mgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "c-cluster",
					},
				},
			},
		},
		{
			name: "test-good-ca",
			args: args{
				conditionSet:           true,
				expectedConditionValue: "True",
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "stv-aggregation",
						Namespace: namespace.System,
					},
					Data: map[string][]byte{
						CacertsValid: []byte("true"),
						"ca.crt":     []byte("test"),
					},
				},
				cluster: &mgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "c-cluster",
					},
				},
			},
		},
		{
			name: "test-bad-ca",
			args: args{
				conditionSet:           true,
				expectedConditionValue: "False",
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "stv-aggregation",
						Namespace: namespace.System,
					},
					Data: map[string][]byte{
						CacertsValid: []byte("false"),
						"ca.crt":     []byte("test"),
					},
				},
				cluster: &mgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "c-cluster",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newMocks(t)
			cav := &CertificateAuthorityValidator{
				clusterName:   tt.args.cluster.Name,
				clusterLister: mocks.mockClusterLister,
				clusters:      mocks.mockCluster,
				secrets:       mocks.mockSecret,
			}

			a := assert.New(t)

			if tt.args.secret != nil && tt.args.secret.Name == "stv-aggregation" && tt.args.secret.Namespace == namespace.System {
				mocks.mockClusterLister.EXPECT().Get("", tt.args.cluster.Name).Return(tt.args.cluster, nil)
			}

			if tt.args.conditionSet {
				mocks.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
					if tt.args.conditionSet {
						require.Len(t, cluster.Status.Conditions, 1)
						require.Equal(t, string(CertificateAuthorityValid), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
						require.Equal(t, tt.args.expectedConditionValue, string(cluster.Status.Conditions[0].Status), "expected condition value to be set")
					} else {
						require.Len(t, cluster.Status.Conditions, 0)
					}
					return cluster, nil
				})
			}

			_, err := cav.onStvAggregationSecret(tt.args.secret.Name, tt.args.secret)
			a.NoError(err)
		})
	}
}
