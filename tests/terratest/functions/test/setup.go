package functions

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	set "github.com/rancher/rancher/tests/terratest/functions/set"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Setup(t *testing.T) *terraform.Options {
	clusterConfig := new(terratest.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	// Set initial infrastructure by building TFs declarative config file - [main.tf]
	successful, err := set.SetConfigTF(clusterConfig.KubernetesVersion, clusterConfig.Nodepools)
	require.NoError(t, err)
	assert.Equal(t, true, successful)

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{

		TerraformDir: "../../../../terratest/modules/cluster",
		NoColor:      true,
	})

	return terraformOptions
}
