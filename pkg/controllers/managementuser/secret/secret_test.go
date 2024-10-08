package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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
