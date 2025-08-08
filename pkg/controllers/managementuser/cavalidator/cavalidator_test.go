package cavalidator

import (
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
			ctrl := gomock.NewController(t)
			mockClusterLister := fake.NewMockNonNamespacedCacheInterface[*mgmtv3.Cluster](ctrl)
			if tt.args.secret != nil && tt.args.secret.Name == "stv-aggregation" && tt.args.secret.Namespace == namespace.System {
				mockClusterLister.EXPECT().Get(tt.args.cluster.Name).Return(tt.args.cluster, nil)
			}

			mockCluster := fake.NewMockNonNamespacedClientInterface[*mgmtv3.Cluster, *mgmtv3.ClusterList](ctrl)
			if tt.args.conditionSet {
				mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
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

			cav := &CertificateAuthorityValidator{
				clusterName:  tt.args.cluster.Name,
				clusterCache: mockClusterLister,
				clusters:     mockCluster,
			}

			secret := tt.args.secret
			_, err := cav.onStvAggregationSecret(secret.Namespace+"/"+secret.Name, secret)
			assert.NoError(t, err)
		})
	}
}
