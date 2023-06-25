package terratest

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BuildModuleTestSuite struct {
	suite.Suite
}

func (r *BuildModuleTestSuite) TestBuildModule() (bool, error) {
	r.T().Parallel()

	defer cleanup.CleanupConfigTF(r.T())

	result, err := terratest.BuildModule(r.T())
	require.NoError(r.T(), err)
	assert.Equal(r.T(), true, result)

	return result, nil
}

func TestBuildModuleTestSuite(t *testing.T) {
	suite.Run(t, new(BuildModuleTestSuite))
}
