package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	format "github.com/rancher/rancher/tests/terratest/functions/format"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/zclconf/go-cty/cty"
)

func SetRKE2K3s(k8sVersion string, nodePools []terratest.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	// create new empty hcl file object
	f := hclwrite.NewEmptyFile()

	// initialize the body of the new file object
	rootBody := f.Body()

	// initialize terraform object and set req provider version
	tfBlock := rootBody.AppendNewBlock("terraform", nil)
	tfBlockBody := tfBlock.Body()
	reqProvsBlock := tfBlockBody.AppendNewBlock("required_providers", nil)
	reqProvsBlockBody := reqProvsBlock.Body()

	reqProvsBlockBody.SetAttributeValue("rancher2", cty.ObjectVal(map[string]cty.Value{
		"source":  cty.StringVal("rancher/rancher2"),
		"version": cty.StringVal(terraformConfig.ProviderVersion),
	}))
	rootBody.AppendNewline()

	// Provider
	provBlock := rootBody.AppendNewBlock("provider", []string{"rancher2"})
	provBlockBody := provBlock.Body()
	provBlockBody.SetAttributeValue("api_url", cty.StringVal(`https://`+rancherConfig.Host))
	provBlockBody.SetAttributeValue("token_key", cty.StringVal(rancherConfig.AdminToken))
	provBlockBody.SetAttributeValue("insecure", cty.BoolVal(*rancherConfig.Insecure))
	rootBody.AppendNewline()

	// Resource cloud credential
	cloudCredBlock := rootBody.AppendNewBlock("data", []string{"rancher2_cloud_credential", "rancher2_cloud_credential"})
	cloudCredBlockBody := cloudCredBlock.Body()
	cloudCredBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.CloudCredentialName))
	rootBody.AppendNewline()

	// Resource machine config
	machineConfigBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_machine_config_v2", "rancher2_machine_config_v2"})
	machineConfigBlockBody := machineConfigBlock.Body()
	machineConfigBlockBody.SetAttributeValue("generate_name", cty.StringVal(terraformConfig.MachineConfigName))

	// amazonec2_config
	if terraformConfig.Module == "ec2_rke2" || terraformConfig.Module == "ec2_k3s" {
		ec2ConfigBlock := machineConfigBlockBody.AppendNewBlock("amazonec2_config", nil)
		ec2ConfigBlockBody := ec2ConfigBlock.Body()
		ec2ConfigBlockBody.SetAttributeValue("ami", cty.StringVal(terraformConfig.Ami))
		ec2ConfigBlockBody.SetAttributeValue("region", cty.StringVal(terraformConfig.Region))
		awsSecGroupNames := format.ListOfStrings(terraformConfig.AWSSecurityGroupNames)
		ec2ConfigBlockBody.SetAttributeRaw("security_group", awsSecGroupNames)
		ec2ConfigBlockBody.SetAttributeValue("subnet_id", cty.StringVal(terraformConfig.AWSSubnetID))
		ec2ConfigBlockBody.SetAttributeValue("vpc_id", cty.StringVal(terraformConfig.AWSVpcID))
		ec2ConfigBlockBody.SetAttributeValue("zone", cty.StringVal(terraformConfig.AWSZoneLetter))
	}

	// linode_config
	if terraformConfig.Module == "linode_rke2" || terraformConfig.Module == "linode_k3s" {
		linodeConfigBlock := machineConfigBlockBody.AppendNewBlock("linode_config", nil)
		linodeConfigBlockBody := linodeConfigBlock.Body()
		linodeConfigBlockBody.SetAttributeValue("image", cty.StringVal(terraformConfig.LinodeImage))
		linodeConfigBlockBody.SetAttributeValue("region", cty.StringVal(terraformConfig.Region))
		linodeConfigBlockBody.SetAttributeValue("root_pass", cty.StringVal(terraformConfig.LinodeRootPass))
		linodeConfigBlockBody.SetAttributeValue("token", cty.StringVal(terraformConfig.LinodeToken))
	}

	rootBody.AppendNewline()

	// Resource cluster_v2
	clusterBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cluster_v2", "rancher2_cluster_v2"})
	clusterBlockBody := clusterBlock.Body()
	clusterBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.ClusterName))
	clusterBlockBody.SetAttributeValue("kubernetes_version", cty.StringVal(k8sVersion))
	clusterBlockBody.SetAttributeValue("enable_network_policy", cty.BoolVal(terraformConfig.EnableNetworkPolicy))
	clusterBlockBody.SetAttributeValue("default_cluster_role_for_project_members", cty.StringVal(terraformConfig.DefaultClusterRoleForProjectMembers))
	rkeConfigBlock := clusterBlockBody.AppendNewBlock("rke_config", nil)
	rkeConfigBlockBody := rkeConfigBlock.Body()

	// Resource machine pools
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		if !pool.Etcd && !pool.Cp && !pool.Wkr {
			return false, fmt.Errorf(`no roles selected for pool` + poolNum + `; at least one role is required`)
		}
		if pool.Quantity <= 0 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 0`)
		}
		machinePoolsBlock := rkeConfigBlockBody.AppendNewBlock("machine_pools", nil)
		machinePoolsBlockBody := machinePoolsBlock.Body()
		machinePoolsBlockBody.SetAttributeValue("name", cty.StringVal(`pool`+poolNum))
		cloudCredSecretName := hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(`data.rancher2_cloud_credential.rancher2_cloud_credential.id`)},
		}
		machinePoolsBlockBody.SetAttributeRaw("cloud_credential_secret_name", cloudCredSecretName)
		machinePoolsBlockBody.SetAttributeValue("control_plane_role", cty.BoolVal(pool.Cp))
		machinePoolsBlockBody.SetAttributeValue("etcd_role", cty.BoolVal(pool.Etcd))
		machinePoolsBlockBody.SetAttributeValue("worker_role", cty.BoolVal(pool.Wkr))
		machinePoolsBlockBody.SetAttributeValue("quantity", cty.NumberIntVal(pool.Quantity))
		machineConfigBlock := machinePoolsBlockBody.AppendNewBlock("machine_config", nil)
		machineConfigBlockBody := machineConfigBlock.Body()
		kind := hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_machine_config_v2.rancher2_machine_config_v2.kind`)},
		}
		machineConfigBlockBody.SetAttributeRaw("kind", kind)
		name := hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_machine_config_v2.rancher2_machine_config_v2.name`)},
		}
		machineConfigBlockBody.SetAttributeRaw("name", name)

		num++
	}

	// Write hcl file
	file.Write(f.Bytes())

	if err != nil {
		return false, err
	}
	return true, nil
}
