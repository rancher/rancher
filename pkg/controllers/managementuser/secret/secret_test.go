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
				syncAnnotation:                          "true",
				syncNamespaceAnnotation:                 "kube-system",
				syncNameAnnotation:                      "resource-sync-test",
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
