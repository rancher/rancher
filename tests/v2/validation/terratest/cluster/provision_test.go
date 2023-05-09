package terratest

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProvisionTestSuite struct {
	suite.Suite
}

func (r *ProvisionTestSuite) TestProvision() (bool, error) {
	r.T().Parallel()

	terraformOptions, result, err := terratest.Setup(r.T())
	require.NoError(r.T(), err)
	assert.Equal(r.T(), true, result)

	defer cleanup.Cleanup(r.T(), terraformOptions)

	_, error_1 := terratest.Provision(r.T(), terraformOptions)
	require.NoError(r.T(), error_1)
	assert.Equal(r.T(), true, result)

	return result, nil
}

func TestProvisionTestSuite(t *testing.T) {
	suite.Run(t, new(ProvisionTestSuite))
}