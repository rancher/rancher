package httpproxy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var TestCases = []struct{ host, service, region string }{
	{"ec2.us-west-2.amazonaws.com", "ec2", "us-west-2"},
	{"eks.eu-central-1.amazonaws.com", "eks", "eu-central-1"},
	{"iam.amazonaws.com", "iam", "us-east-1"},
}

func TestGetServiceAndRegion(t *testing.T) {
	signer := awsv4{}

	for _, testCase := range TestCases {
		service, region := signer.getServiceAndRegion(testCase.host)
		fmt.Printf("Host: %s Service: %s Region: %s\n", testCase.host, service, region)
		assert.Equal(t, testCase.service, service)
		assert.Equal(t, testCase.region, region)
	}
}

func TestHeaderInjectSignSetsRequestHeaders(t *testing.T) {
	req := makeRequest(nil)
	sg := makeSecretGetter(map[string]string{
		"token": "abc123",
		"user":  "alice",
	})
	auth := "headerinject credID=cattle-global-data/my-cred headers=X-Token=token;X-User=user"

	err := headerinject{}.sign(req, sg, auth)
	require.NoError(t, err)
	assert.Equal(t, "abc123", req.Header.Get("X-Token"))
	assert.Equal(t, "alice", req.Header.Get("X-User"))
}
