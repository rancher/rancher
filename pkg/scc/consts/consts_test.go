package consts

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSCCCredentialsSecretName(t *testing.T) {
	assert.Equal(t, "scc-system-credentials-", SCCCredentialsSecretName(""))
	assert.Equal(t, "scc-system-credentials-test", SCCCredentialsSecretName("test"))
}
