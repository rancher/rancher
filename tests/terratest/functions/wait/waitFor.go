package functions

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	wait_action "github.com/rancher/rancher/tests/terratest/functions/wait/action"
	wait_state "github.com/rancher/rancher/tests/terratest/functions/wait/state"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
)

func WaitFor(t *testing.T, client *rancher.Client, clusterID string, action string) {
	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	module := terraformConfig.Module

	if module == "aks" || module == "eks" || module == "ec2_k3s" || module == "ec2_rke1" || module == "ec2_rke2" || module == "linode_k3s" || module == "linode_rke1" || module == "linode_rke2" {
		if module != "eks" && !((module == "ec2_rke1" || module == "linode_rke1") && action == "kubernetes-upgrade") {
			wait_state.WaitingOrUpdating(t, client, clusterID)
		}

		wait_state.ActiveAndReady(t, client, clusterID)

		if action == "scale-up" || action == "kubernetes-upgrade" {
			wait_state.ActiveNodes(t, client, clusterID)

			wait_state.ActiveAndReady(t, client, clusterID)
		}
	}

	if action == "scale-up" {
		wait_action.ScaleUp(t, client, clusterID)
		wait_state.ActiveAndReady(t, client, clusterID)
	}

	if action == "scale-down" {
		wait_action.ScaleDown(t, client, clusterID)
		wait_state.ActiveAndReady(t, client, clusterID)
	}

	if action == "kubernetes-upgrade" {
		wait_action.KubernetesUpgrade(t, client, clusterID, module)
		wait_state.ActiveAndReady(t, client, clusterID)
	}

}
