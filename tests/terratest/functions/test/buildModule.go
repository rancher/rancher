package functions

import (
	"os"
	"testing"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	set "github.com/rancher/rancher/tests/terratest/functions/set"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BuildModule(t *testing.T) (bool, error) {
	clusterConfig := new(terratest.TerratestConfig)
	config.LoadConfig("terratest", clusterConfig)

	// Set initial infrastructure by building TFs declarative config file - [main.tf]
	successful, err := set.SetConfigTF(clusterConfig.KubernetesVersion, clusterConfig.Nodepools)
	require.NoError(t, err)
	assert.Equal(t, true, successful)

	// Log module
	module, err := os.ReadFile("../../../../terratest/modules/cluster/main.tf")

	if err != nil {
		return false, err
	}

	t.Log(string(module))

	return true, nil
}
