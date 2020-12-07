package eks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEC2ServiceEndpoint(t *testing.T) {
	assert := assert.New(t)

	endpoint := getEC2ServiceEndpoint("us-east-2")
	assert.Equal("ec2.amazonaws.com", endpoint)

	endpoint = getEC2ServiceEndpoint("eu-central-1")
	assert.Equal("ec2.amazonaws.com", endpoint)

	endpoint = getEC2ServiceEndpoint("cn-north-1")
	assert.Equal("ec2.amazonaws.com.cn", endpoint)

	endpoint = getEC2ServiceEndpoint("cn-northwest-1")
	assert.Equal("ec2.amazonaws.com.cn", endpoint)
}
