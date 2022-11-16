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
	"github.com/rancher/rancher/tests/terratest/tests"
	"github.com/zclconf/go-cty/cty"
)

func SetAKS(k8sVersion string, nodePools []tests.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(tests.TerraformConfig)
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
	cloudCredBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cloud_credential", "rancher2_cloud_credential"})
	cloudCredBlockBody := cloudCredBlock.Body()
	cloudCredBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.CloudCredentialName))
	azCredConfigBlock := cloudCredBlockBody.AppendNewBlock("azure_credential_config", nil)
	azCredConfigBlockBody := azCredConfigBlock.Body()
	azCredConfigBlockBody.SetAttributeValue("client_id", cty.StringVal(terraformConfig.AzureClientID))
	azCredConfigBlockBody.SetAttributeValue("client_secret", cty.StringVal(terraformConfig.AzureClientSecret))
	azCredConfigBlockBody.SetAttributeValue("subscription_id", cty.StringVal(terraformConfig.AzureSubscriptionID))
	rootBody.AppendNewline()

	// Resource cluster
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

	// Resource node pools
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		if pool.Quantity <= 0 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 0`)
		}
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

	// Write hcl file
	file.Write(f.Bytes())

	if err != nil {
		return false, err
	}
	return true, nil
}
