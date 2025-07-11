package httpproxy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
