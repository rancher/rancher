package functions

import (
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	format "github.com/rancher/rancher/tests/terratest/functions/format"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func SetEKS(t *testing.T, k8sVersion string, nodePools []terratest.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	testSession := session.NewSession()

	client, err := rancher.NewClient("", testSession)
	require.NoError(t, err)

	f := hclwrite.NewEmptyFile()

	rootBody := f.Body()

	tfBlock := rootBody.AppendNewBlock("terraform", nil)
	tfBlockBody := tfBlock.Body()

	reqProvsBlock := tfBlockBody.AppendNewBlock("required_providers", nil)
	reqProvsBlockBody := reqProvsBlock.Body()

	reqProvsBlockBody.SetAttributeValue("rancher2", cty.ObjectVal(map[string]cty.Value{
		"source":  cty.StringVal("rancher/rancher2"),
		"version": cty.StringVal(terraformConfig.ProviderVersion),
	}))

	rootBody.AppendNewline()

	provBlock := rootBody.AppendNewBlock("provider", []string{"rancher2"})
	provBlockBody := provBlock.Body()

	provBlockBody.SetAttributeValue("api_url", cty.StringVal(`https://`+rancherConfig.Host))
	provBlockBody.SetAttributeValue("token_key", cty.StringVal(rancherConfig.AdminToken))
	provBlockBody.SetAttributeValue("insecure", cty.BoolVal(*rancherConfig.Insecure))

	rootBody.AppendNewline()

	cloudCredBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cloud_credential", "rancher2_cloud_credential"})
	cloudCredBlockBody := cloudCredBlock.Body()

	cloudCredBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.CloudCredentialName))

	ec2CredConfigBlock := cloudCredBlockBody.AppendNewBlock("amazonec2_credential_config", nil)
	ec2CredConfigBlockBody := ec2CredConfigBlock.Body()

	ec2CredConfigBlockBody.SetAttributeValue("access_key", cty.StringVal(terraformConfig.AWSAccessKey))
	ec2CredConfigBlockBody.SetAttributeValue("secret_key", cty.StringVal(terraformConfig.AWSSecretKey))
	ec2CredConfigBlockBody.SetAttributeValue("default_region", cty.StringVal(terraformConfig.Region))

	rootBody.AppendNewline()

	clusterBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cluster", "rancher2_cluster"})
	clusterBlockBody := clusterBlock.Body()

	clusterBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.ClusterName))

	eksConfigBlock := clusterBlockBody.AppendNewBlock("eks_config_v2", nil)
	eksConfigBlockBody := eksConfigBlock.Body()

	cloudCredID := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_cloud_credential.rancher2_cloud_credential.id`)},
	}

	eksConfigBlockBody.SetAttributeRaw("cloud_credential_id", cloudCredID)
	eksConfigBlockBody.SetAttributeValue("region", cty.StringVal(terraformConfig.Region))
	eksConfigBlockBody.SetAttributeValue("kubernetes_version", cty.StringVal(k8sVersion))
	awsSubnetsList := format.ListOfStrings(terraformConfig.AWSSubnets)
	eksConfigBlockBody.SetAttributeRaw("subnets", awsSubnetsList)
	awsSecGroupsList := format.ListOfStrings(terraformConfig.AWSSecurityGroups)
	eksConfigBlockBody.SetAttributeRaw("security_groups", awsSecGroupsList)
	eksConfigBlockBody.SetAttributeValue("private_access", cty.BoolVal(terraformConfig.PrivateAccess))
	eksConfigBlockBody.SetAttributeValue("public_access", cty.BoolVal(terraformConfig.PublicAccess))

	num := 1
	for _, pool := range nodePools {
		rancher_server_version, err := client.Management.Setting.ByID("server-version")
		require.NoError(t, err)
		major_minor_server_version := rancher_server_version.Value[0:4]

		poolNum := strconv.Itoa(num)

		result, err := SetResourceNodepoolValidation(t, pool, poolNum)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		nodePoolsBlock := eksConfigBlockBody.AppendNewBlock("node_groups", nil)
		nodePoolsBlockBody := nodePoolsBlock.Body()

		nodePoolsBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.HostnamePrefix+`-pool`+poolNum))
		nodePoolsBlockBody.SetAttributeValue("instance_type", cty.StringVal(pool.InstanceType))
		nodePoolsBlockBody.SetAttributeValue("desired_size", cty.NumberIntVal(pool.DesiredSize))
		nodePoolsBlockBody.SetAttributeValue("max_size", cty.NumberIntVal(pool.MaxSize))
		nodePoolsBlockBody.SetAttributeValue("min_size", cty.NumberIntVal(pool.MinSize))

		if major_minor_server_version != "v2.6" {
			nodePoolsBlockBody.SetAttributeValue("node_role", cty.StringVal(pool.NodeRole))
		}

		num++
	}

	_, err = file.Write(f.Bytes())

	if err != nil {
		t.Logf("Failed to write EKS configurations to main.tf file. Error: %v", err)
		return false, err
	}

	return true, nil
}
