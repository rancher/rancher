package functions

import (
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	format "github.com/rancher/rancher/tests/terratest/functions/format"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func SetRKE1(t *testing.T, k8sVersion string, nodePools []terratest.Nodepool, file *os.File) (done bool, err error) {
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

	nodeTemplateBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_node_template", "rancher2_node_template"})
	nodeTemplateBlockBody := nodeTemplateBlock.Body()

	nodeTemplateBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.NodeTemplateName))

	if terraformConfig.Module == "ec2_rke1" {
		ec2ConfigBlock := nodeTemplateBlockBody.AppendNewBlock("amazonec2_config", nil)
		ec2ConfigBlockBody := ec2ConfigBlock.Body()

		ec2ConfigBlockBody.SetAttributeValue("access_key", cty.StringVal(terraformConfig.AWSAccessKey))
		ec2ConfigBlockBody.SetAttributeValue("secret_key", cty.StringVal(terraformConfig.AWSSecretKey))
		ec2ConfigBlockBody.SetAttributeValue("ami", cty.StringVal(terraformConfig.Ami))
		ec2ConfigBlockBody.SetAttributeValue("region", cty.StringVal(terraformConfig.Region))
		awsSecGroupNames := format.ListOfStrings(terraformConfig.AWSSecurityGroupNames)
		ec2ConfigBlockBody.SetAttributeRaw("security_group", awsSecGroupNames)
		ec2ConfigBlockBody.SetAttributeValue("subnet_id", cty.StringVal(terraformConfig.AWSSubnetID))
		ec2ConfigBlockBody.SetAttributeValue("vpc_id", cty.StringVal(terraformConfig.AWSVpcID))
		ec2ConfigBlockBody.SetAttributeValue("zone", cty.StringVal(terraformConfig.AWSZoneLetter))
		ec2ConfigBlockBody.SetAttributeValue("root_size", cty.NumberIntVal(terraformConfig.AWSRootSize))
		ec2ConfigBlockBody.SetAttributeValue("instance_type", cty.StringVal(terraformConfig.AWSInstanceType))
	}

	if terraformConfig.Module == "linode_rke1" {
		linodeConfigBlock := nodeTemplateBlockBody.AppendNewBlock("linode_config", nil)
		linodeConfigBlockBody := linodeConfigBlock.Body()

		linodeConfigBlockBody.SetAttributeValue("image", cty.StringVal(terraformConfig.LinodeImage))
		linodeConfigBlockBody.SetAttributeValue("region", cty.StringVal(terraformConfig.Region))
		linodeConfigBlockBody.SetAttributeValue("root_pass", cty.StringVal(terraformConfig.LinodeRootPass))
		linodeConfigBlockBody.SetAttributeValue("token", cty.StringVal(terraformConfig.LinodeToken))
	}

	rootBody.AppendNewline()

	clusterBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cluster", "rancher2_cluster"})
	clusterBlockBody := clusterBlock.Body()

	dependsOnTemp := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`[rancher2_node_template.rancher2_node_template]`)},
	}

	clusterBlockBody.SetAttributeRaw("depends_on", dependsOnTemp)
	clusterBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.ClusterName))

	rkeConfigBlock := clusterBlockBody.AppendNewBlock("rke_config", nil)
	rkeConfigBlockBody := rkeConfigBlock.Body()

	rkeConfigBlockBody.SetAttributeValue("kubernetes_version", cty.StringVal(k8sVersion))

	networkBlock := rkeConfigBlockBody.AppendNewBlock("network", nil)
	networkBlockBody := networkBlock.Body()

	networkBlockBody.SetAttributeValue("plugin", cty.StringVal(terraformConfig.NetworkPlugin))

	rootBody.AppendNewline()

	clusterSyncNodePoolIDs := ``
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)

		result, err := SetResourceNodepoolValidation(t, pool, poolNum)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		nodePoolBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_node_pool", `pool` + poolNum})
		nodePoolBlockBody := nodePoolBlock.Body()

		dependsOnCluster := hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(`[rancher2_cluster.rancher2_cluster]`)},
		}

		nodePoolBlockBody.SetAttributeRaw("depends_on", dependsOnCluster)

		clusterID := hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_cluster.rancher2_cluster.id`)},
		}

		nodePoolBlockBody.SetAttributeRaw("cluster_id", clusterID)
		nodePoolBlockBody.SetAttributeValue("name", cty.StringVal(`pool`+poolNum))
		nodePoolBlockBody.SetAttributeValue("hostname_prefix", cty.StringVal(terraformConfig.HostnamePrefix+`-pool`+poolNum+`-`))

		nodeTempID := hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_node_template.rancher2_node_template.id`)},
		}

		nodePoolBlockBody.SetAttributeRaw("node_template_id", nodeTempID)
		nodePoolBlockBody.SetAttributeValue("quantity", cty.NumberIntVal(pool.Quantity))
		nodePoolBlockBody.SetAttributeValue("control_plane", cty.BoolVal(pool.Controlplane))
		nodePoolBlockBody.SetAttributeValue("etcd", cty.BoolVal(pool.Etcd))
		nodePoolBlockBody.SetAttributeValue("worker", cty.BoolVal(pool.Worker))

		rootBody.AppendNewline()

		if num != len(nodePools) {
			clusterSyncNodePoolIDs = clusterSyncNodePoolIDs + `rancher2_node_pool.pool` + poolNum + `.id, `
		}
		if num == len(nodePools) {
			clusterSyncNodePoolIDs = clusterSyncNodePoolIDs + `rancher2_node_pool.pool` + poolNum + `.id`
		}

		num++
	}

	clusterSyncBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cluster_sync", "rancher2_cluster_sync"})
	clusterSyncBlockBody := clusterSyncBlock.Body()

	clusterID := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_cluster.rancher2_cluster.id`)},
	}

	clusterSyncBlockBody.SetAttributeRaw("cluster_id", clusterID)

	nodePoolIDs := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`[` + clusterSyncNodePoolIDs + `]`)},
	}

	clusterSyncBlockBody.SetAttributeRaw("node_pool_ids", nodePoolIDs)
	clusterSyncBlockBody.SetAttributeValue("state_confirm", cty.NumberIntVal(2))

	rootBody.AppendNewline()

	_, err = file.Write(f.Bytes())

	if err != nil {
		t.Logf("Failed to write RKE1 configurations to main.tf file. Error: %v", err)
		return false, err
	}

	return true, nil
}