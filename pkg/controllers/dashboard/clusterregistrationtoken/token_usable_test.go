package clusterregistrationtoken

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestTokenSecretCluster(t *testing.T) {
	now := metav1.Now()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "crt-token-default-token", Namespace: "c-m-test"}}
	activeNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}
	namespaceNotFound := apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "c-m-test")
	itsCluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}

	tests := []struct {
		name         string
		namespace    *corev1.Namespace
		namespaceErr error
		cluster      *v3.Cluster
		clusterErr   error
		wantCluster  *v3.Cluster
		wantStale    bool
		wantErr      bool
	}{
		{
			name:        "active namespace",
			namespace:   activeNamespace,
			cluster:     itsCluster,
			wantCluster: itsCluster,
		},
		{
			name:      "namespace being deleted",
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", DeletionTimestamp: &now}},
			cluster:   itsCluster,
			wantStale: true,
		},
		{
			name:      "namespace terminating",
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating}},
			cluster:   itsCluster,
			wantStale: true,
		},
		{
			// A secret can't outlive its namespace: the namespace cache is behind.
			name:         "namespace not found",
			namespaceErr: namespaceNotFound,
			cluster:      itsCluster,
			wantCluster:  itsCluster,
		},
		{
			// No other cluster has the name: callers decide whether they can act without a cluster.
			name:       "cluster not found",
			namespace:  activeNamespace,
			clusterErr: apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, "c-m-test"),
		},
		{
			name:         "namespace lookup failure",
			namespaceErr: errors.New("boom"),
			cluster:      itsCluster,
			wantErr:      true,
		},
		{
			name:       "cluster lookup failure",
			namespace:  activeNamespace,
			clusterErr: errors.New("boom"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, stale, err := TokenSecretCluster(secret, func(name string) (*corev1.Namespace, error) {
				assert.Equal(t, "c-m-test", name)
				return tt.namespace, tt.namespaceErr
			}, func(name string) (*v3.Cluster, error) {
				assert.Equal(t, "c-m-test", name)
				return tt.cluster, tt.clusterErr
			})
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, cluster, "a token must never be accepted when it can't be checked")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStale, stale)
			assert.Same(t, tt.wantCluster, cluster, "the cluster the token was checked against")
		})
	}
}
