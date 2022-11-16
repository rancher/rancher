package functions

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	cleanup "github.com/rancher/rancher/tests/terratest/functions/cleanup"
)

func ForceCleanup(t *testing.T) {
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../../modules/cluster",
		NoColor:      true,
	})

	terraform.Destroy(t, terraformOptions)
	cleanup.CleanupConfigTF()
}
