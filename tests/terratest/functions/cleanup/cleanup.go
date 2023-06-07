package functions

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

func Cleanup(t *testing.T, terraformOptions *terraform.Options) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	if *rancherConfig.Cleanup {
		terraform.Destroy(t, terraformOptions)
		CleanupConfigTF(t)
	}
}
