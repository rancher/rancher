package functions

import (
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	format "github.com/josh-diamond/rancher/tests/terratest/functions/format"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func SetAKS(t *testing.T, k8sVersion string, nodePools []terratest.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

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

	azCredConfigBlock := cloudCredBlockBody.AppendNewBlock("azure_credential_config", nil)
	azCredConfigBlockBody := azCredConfigBlock.Body()

	azCredConfigBlockBody.SetAttributeValue("client_id", cty.StringVal(terraformConfig.AzureClientID))
	azCredConfigBlockBody.SetAttributeValue("client_secret", cty.StringVal(terraformConfig.AzureClientSecret))
	azCredConfigBlockBody.SetAttributeValue("subscription_id", cty.StringVal(terraformConfig.AzureSubscriptionID))

	rootBody.AppendNewline()

	clusterBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cluster", "rancher2_cluster"})
	clusterBlockBody := clusterBlock.Body()

	clusterBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.ClusterName))

	aksConfigBlock := clusterBlockBody.AppendNewBlock("aks_config_v2", nil)
	aksConfigBlockBody := aksConfigBlock.Body()

	cloudCredID := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_cloud_credential.rancher2_cloud_credential.id`)},
	}

	aksConfigBlockBody.SetAttributeRaw("cloud_credential_id", cloudCredID)
	aksConfigBlockBody.SetAttributeValue("resource_group", cty.StringVal(terraformConfig.ResourceGroup))
	aksConfigBlockBody.SetAttributeValue("resource_location", cty.StringVal(terraformConfig.ResourceLocation))
	aksConfigBlockBody.SetAttributeValue("dns_prefix", cty.StringVal(terraformConfig.HostnamePrefix))
	aksConfigBlockBody.SetAttributeValue("kubernetes_version", cty.StringVal(k8sVersion))
	aksConfigBlockBody.SetAttributeValue("network_plugin", cty.StringVal(terraformConfig.NetworkPlugin))

	availabilityZones := format.ListOfStrings(terraformConfig.AvailabilityZones)

	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)

		result, err := SetResourceNodepoolValidation(t, pool, poolNum)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		nodePoolsBlock := aksConfigBlockBody.AppendNewBlock("node_pools", nil)
		nodePoolsBlockBody := nodePoolsBlock.Body()

		nodePoolsBlockBody.SetAttributeRaw("availability_zones", availabilityZones)
		nodePoolsBlockBody.SetAttributeValue("name", cty.StringVal("pool"+poolNum))
		nodePoolsBlockBody.SetAttributeValue("count", cty.NumberIntVal(pool.Quantity))
		nodePoolsBlockBody.SetAttributeValue("orchestrator_version", cty.StringVal(k8sVersion))
		nodePoolsBlockBody.SetAttributeValue("os_disk_size_gb", cty.NumberIntVal(terraformConfig.OSDiskSizeGB))
		nodePoolsBlockBody.SetAttributeValue("vm_size", cty.StringVal(terraformConfig.VMSize))

		num++
	}

	_, err = file.Write(f.Bytes())

	if err != nil {
		t.Logf("Failed to write AKS configurations to main.tf file. Error: %v", err)
		return false, err
	}

	return true, nil
}
