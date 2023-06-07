package terratest

import (
	"testing"

	terratest "github.com/josh-diamond/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CleanupTestSuite struct {
	suite.Suite
}

func (r *CleanupTestSuite) TestCleanup() (bool, error) {
	r.T().Parallel()

	result, err := terratest.ForceCleanup(r.T())
	require.NoError(r.T(), err)
	assert.Equal(r.T(), true, result)

	return result, nil
}

func TestCleanupTestSuite(t *testing.T) {
	suite.Run(t, new(CleanupTestSuite))
}
