package cluster

import (
	"encoding/base64"
	"fmt"
	"testing"

	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
)

func TestGeneratePrivateRegistryDockerConfig(t *testing.T) {
	cfg, err := GeneratePrivateRegistryDockerConfig(nil)
	assert.Nil(t, err)
	assert.Equal(t, "", cfg)

	registry := rketypes.PrivateRegistry{
		URL:      "0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com",
		User:     "testuser",
		Password: "password",
	}
	s := fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, registry.URL, registry.User, registry.Password, base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", registry.User, registry.Password))))
	base64s := base64.URLEncoding.EncodeToString([]byte(s))
	assert.Nil(t, err)

	cfg, err = GeneratePrivateRegistryDockerConfig(&registry)
	assert.Nil(t, err)

	assert.Equal(t, base64s, cfg)
}
