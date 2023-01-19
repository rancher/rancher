package terratest

import (
	"testing"

	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/suite"
)

type CleanupTestSuite struct {
	suite.Suite
}

func (r *CleanupTestSuite) TestCleanup() {
	r.T().Parallel()

	terratest.ForceCleanup(r.T())
}

func TestCleanupTestSuite(t *testing.T) {
	suite.Run(t, new(CleanupTestSuite))
}
