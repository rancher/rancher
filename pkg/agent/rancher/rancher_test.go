package rancher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPreBootstrap(t *testing.T) {
	t.Setenv(preBootstrapEnvVar, "true")
	assert.True(t, isPreBootstrap())

	t.Setenv(preBootstrapEnvVar, "TRUE")
	assert.True(t, isPreBootstrap())

	t.Setenv(preBootstrapEnvVar, "false")
	assert.False(t, isPreBootstrap())

	t.Setenv(preBootstrapEnvVar, "")
	assert.False(t, isPreBootstrap())
}
