package secrets

import (
	"testing"

	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
)

func TestGenericSAMLSecretFields(t *testing.T) {
	assert.Equal(t, []string{client.GenericSAMLConfigFieldSpKey}, TypeToFields[client.GenericSAMLConfigType])
}
