package secret

import (
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInjectClusterIdIntoSecretData(t *testing.T) {
	sec := &corev1.Secret{Data: map[string][]byte{
		"bazqux": []byte("{{clusterId}}"),
	}}

	c := ResourceSyncController{clusterId: "foobar"}

	assert.Equal(t, c.injectClusterIdIntoSecretData(sec).Data["bazqux"], []byte("foobar"))
}

func TestRemoveClusterIdFromSecretData(t *testing.T) {
	sec := &corev1.Secret{Data: map[string][]byte{
		"bazqux": []byte("foobar"),
	}}

	c := ResourceSyncController{clusterId: "foobar"}

	assert.Equal(t, c.removeClusterIdFromSecretData(sec).Data["bazqux"], []byte("{{clusterId}}"))
}

func TestSyncable(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		secret      *corev1.Secret
		want        bool
	}{
		{
			name:        "authorized cluster in allowed namespace",
			clusterName: "cluster-b",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.SyncPreBootstrapAnnotation: "true",
						capr.SyncNamespaceAnnotation:    "kube-system",
						capr.AuthorizedObjectAnnotation: "cluster-a,cluster-b",
					},
				},
			},
			want: true,
		},
		{
			name:        "unauthorized cluster",
			clusterName: "cluster-b",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.SyncPreBootstrapAnnotation: "true",
						capr.SyncNamespaceAnnotation:    "kube-system",
						capr.AuthorizedObjectAnnotation: "cluster-a",
					},
				},
			},
			want: false,
		},
		{
			name:        "disallowed namespace",
			clusterName: "cluster-b",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.SyncPreBootstrapAnnotation: "true",
						capr.SyncNamespaceAnnotation:    "default",
						capr.AuthorizedObjectAnnotation: "cluster-a,cluster-b",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := ResourceSyncController{clusterName: tt.clusterName}
			assert.Equal(t, tt.want, controller.syncable(tt.secret))
		})
	}
}

func TestSyncableIgnoresClusterSelectorAnnotation(t *testing.T) {
	c := ResourceSyncController{clusterName: "test-cluster"}
	tests := []struct {
		name          string
		authorization string
		want          bool
	}{
		{
			name: "empty selector does not authorize resource sync",
		},
		{
			name:          "name authorization allows resource sync",
			authorization: "test-cluster",
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{
				capr.SyncAnnotation:                     "true",
				capr.SyncNamespaceAnnotation:            "kube-system",
				capr.SyncNameAnnotation:                 "resource-sync-test",
				capr.AuthorizedObjectSelectorAnnotation: "",
			}
			if tt.authorization != "" {
				annotations[capr.AuthorizedObjectAnnotation] = tt.authorization
			}

			sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}
			assert.Equal(t, tt.want, c.syncable(sec))
		})
	}
}
