package tests

import (
	"testing"

	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
	terratest "github.com/rancher/rancher/tests/terratest/functions/test"
	"github.com/stretchr/testify/require"
)

func TestKubernetesUpgrade(t *testing.T) {
	t.Parallel()

	// set up func
	terraformOptions := terratest.Setup(t)

	// clean up
	defer cleanup.Cleanup(t, terraformOptions)

	// provisioning func
	client, err := terratest.Provision(t, terraformOptions)
	require.NoError(t, err)

	// Upgrade kubernetes version
	err = terratest.KubernetesUpgrade(t, terraformOptions, client)
	require.NoError(t, err)
}
