package terratest

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ScaleTestSuite struct {
	suite.Suite
}

func (r *ScaleTestSuite) TestScale() {
	r.T().Parallel()

	// set up func
	terraformOptions := terratest.Setup(r.T())

	// clean up
	defer cleanup.Cleanup(r.T(), terraformOptions)

	// provisioning func
	client, err := terratest.Provision(r.T(), terraformOptions)
	require.NoError(r.T(), err)

	// scale up
	err = terratest.ScaleUp(r.T(), terraformOptions, client)
	require.NoError(r.T(), err)

	// scale down
	err = terratest.ScaleDown(r.T(), terraformOptions, client)
	require.NoError(r.T(), err)
}

func TestScaleTestSuite(t *testing.T) {
	suite.Run(t, new(ScaleTestSuite))
}
