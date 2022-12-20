package terratest

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KubernetesUpgradeTestSuite struct {
	suite.Suite
}

func (r *KubernetesUpgradeTestSuite) TestKubernetesUpgrade() {
	r.T().Parallel()

	// set up func
	terraformOptions := terratest.Setup(r.T())

	// clean up
	defer cleanup.Cleanup(r.T(), terraformOptions)

	// provisioning func
	client, err := terratest.Provision(r.T(), terraformOptions)
	require.NoError(r.T(), err)

	// Upgrade kubernetes version
	err = terratest.KubernetesUpgrade(r.T(), terraformOptions, client)
	require.NoError(r.T(), err)
}

func TestKubernetesUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(KubernetesUpgradeTestSuite))
}
