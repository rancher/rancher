package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestRegistrationFromSecret(t *testing.T) {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{},
		Data: map[string][]byte{
			dataRegCode:          []byte("hello"),
			dataRegistrationType: []byte(v1.RegistrationModeOnline),
		},
	}

	params, err := extraRegistrationParamsFromSecret(sec)
	assert.NoError(t, err)

	assert.NotNil(t, params)
	assert.NotNil(t, params.hash)
	// valid hash

	seenHash := params.hash
	assert.Equal(t, len(seenHash), 32)

	sec.Data[dataRegCode] = []byte("world")
	params2, err := extraRegistrationParamsFromSecret(sec)
	assert.NoError(t, err)
	assert.NotNil(t, params2)
	assert.NotNil(t, params2.hash)
	assert.NotEqual(t, seenHash, params2.hash)

	seenHash = params2.hash
	sec.Data[dataRegistrationType] = []byte(v1.RegistrationModeOffline)
	params3, err := extraRegistrationParamsFromSecret(sec)
	assert.NoError(t, err)
	assert.NotNil(t, params3)
	assert.NotNil(t, params3.hash)
	assert.NotEqual(t, seenHash, params3.hash)

	for _, label := range params.Labels() {
		assert.Less(t, len(label), 63)
	}

	for _, label := range params2.Labels() {
		assert.Less(t, len(label), 63)
	}

	for _, label := range params3.Labels() {
		assert.Less(t, len(label), 63)
	}
}
