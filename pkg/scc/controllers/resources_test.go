package controllers

import (
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRegistrationFromSecret(t *testing.T) {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{},
		Data: map[string][]byte{
			consts.SecretKeyRegistrationCode: []byte("hello"),
			dataKeyRegistrationType:          []byte(v1.RegistrationModeOnline),
		},
	}

	params, err := extractRegistrationParamsFromSecret(sec)
	assert.NoError(t, err)

	assert.NotNil(t, params)
	assert.NotNil(t, params.contentHash)
	// valid hash

	seenHash := params.contentHash
	assert.Equal(t, len(seenHash), 32)

	sec.Data[consts.SecretKeyRegistrationCode] = []byte("world")
	params2, err := extractRegistrationParamsFromSecret(sec)
	assert.NoError(t, err)
	assert.NotNil(t, params2)
	assert.NotNil(t, params2.contentHash)
	assert.NotEqual(t, seenHash, params2.contentHash)

	seenHash = params2.contentHash
	sec.Data[dataKeyRegistrationType] = []byte(v1.RegistrationModeOffline)
	params3, err := extractRegistrationParamsFromSecret(sec)
	assert.NoError(t, err)
	assert.NotNil(t, params3)
	assert.NotNil(t, params3.contentHash)
	assert.NotEqual(t, seenHash, params3.contentHash)

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
