package clustermanager

import (
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestShouldPreBootstrap(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *apimgmtv3.Cluster
		secrets  []*corev1.Secret
		want     bool
		expectNS string
	}{
		{
			name: "authorized sync-bootstrap secret exists",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:        "c-test",
					FleetWorkspaceName: "fleet-default",
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bootstrap-secret",
						Annotations: map[string]string{
							preBootstrapSyncAnnotation:                     "true",
							"rke.cattle.io/object-authorized-for-clusters": "c-test",
						},
					},
				},
			},
			want:     true,
			expectNS: "fleet-default",
		},
		{
			name: "sync-bootstrap secret not authorized for cluster",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:        "c-test",
					FleetWorkspaceName: "fleet-default",
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bootstrap-secret",
						Annotations: map[string]string{
							preBootstrapSyncAnnotation:                     "true",
							"rke.cattle.io/object-authorized-for-clusters": "c-other",
						},
					},
				},
			},
			want:     false,
			expectNS: "fleet-default",
		},
		{
			name: "no sync-bootstrap secret",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:        "c-test",
					FleetWorkspaceName: "fleet-default",
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "non-bootstrap-secret",
						Annotations: map[string]string{
							"rke.cattle.io/object-authorized-for-clusters": "c-test",
						},
					},
				},
			},
			want:     false,
			expectNS: "fleet-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					assert.Equal(t, tt.expectNS, namespace)
					return tt.secrets, nil
				},
			}

			m := &Manager{secretLister: lister}
			shouldPreBootstrap, err := m.shouldPreBootstrap(tt.cluster)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, shouldPreBootstrap)
		})
	}
}

func TestShouldPreBootstrapWhenClusterAlreadyPreBootstrapped(t *testing.T) {
	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			DisplayName:        "c-test",
			FleetWorkspaceName: "fleet-default",
		},
	}
	apimgmtv3.ClusterConditionPreBootstrapped.True(cluster)

	lister := &corefakes.SecretListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
			t.Fatal("secret list should not be called for already pre-bootstrapped clusters")
			return nil, nil
		},
	}

	m := &Manager{secretLister: lister}
	shouldPreBootstrap, err := m.shouldPreBootstrap(cluster)
	assert.NoError(t, err)
	assert.False(t, shouldPreBootstrap)
}

func TestShouldPreBootstrapWhenSecretListFails(t *testing.T) {
	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			DisplayName:        "c-test",
			FleetWorkspaceName: "fleet-default",
		},
	}

	lister := &corefakes.SecretListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
			return nil, errors.New("list failed")
		},
	}

	m := &Manager{secretLister: lister}
	shouldPreBootstrap, err := m.shouldPreBootstrap(cluster)
	assert.Error(t, err)
	assert.False(t, shouldPreBootstrap)
}

func TestClusterAuthorizedForSecret(t *testing.T) {
	assert.True(t, clusterAuthorizedForSecret("a,b,c-test", "c-test"))
	assert.True(t, clusterAuthorizedForSecret("a, c-test", "c-test"))
	assert.False(t, clusterAuthorizedForSecret("a,b", "c-test"))
	assert.False(t, clusterAuthorizedForSecret("", "c-test"))
}
