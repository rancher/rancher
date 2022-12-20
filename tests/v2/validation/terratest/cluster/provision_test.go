package terratest

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProvisionTestSuite struct {
	suite.Suite
}

func (r *ProvisionTestSuite) TestProvision() {
	r.T().Parallel()

	// set up func
	terraformOptions := terratest.Setup(r.T())

	// clean up
	defer cleanup.Cleanup(r.T(), terraformOptions)

	// provisioning func
	_, err := terratest.Provision(r.T(), terraformOptions)
	require.NoError(r.T(), err)
}

func TestProvisionTestSuite(t *testing.T) {
	suite.Run(t, new(ProvisionTestSuite))
}
