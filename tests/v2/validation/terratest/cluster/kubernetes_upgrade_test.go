package terratest

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KubernetesUpgradeTestSuite struct {
	suite.Suite
}

func (r *KubernetesUpgradeTestSuite) TestKubernetesUpgrade() (bool, error) {
	r.T().Parallel()

	terraformOptions, result, err := terratest.Setup(r.T())
	require.NoError(r.T(), err)
	assert.Equal(r.T(), true, result)

	defer cleanup.Cleanup(r.T(), terraformOptions)

	client, err := terratest.Provision(r.T(), terraformOptions)
	require.NoError(r.T(), err)

	err = terratest.KubernetesUpgrade(r.T(), terraformOptions, client)
	require.NoError(r.T(), err)

	return result, nil
}

func TestKubernetesUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(KubernetesUpgradeTestSuite))
}