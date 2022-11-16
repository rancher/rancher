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

func SetRKE1(k8sVersion string, nodePools []tests.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(tests.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	// create new empty hcl file object
	f := hclwrite.NewEmptyFile()

	// initialize the body of the new file object
	rootBody := f.Body()

	// initialize terraform object and set req provider/version
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

	// Resource node template
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

	// Resource cluster
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

	// Resource node pools
	clusterSyncNodePoolIDs := ``
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		if !pool.Etcd && !pool.Cp && !pool.Wkr {
			return false, fmt.Errorf(`no roles selected for pool` + poolNum + `; at least one role is required`)
		}
		if pool.Quantity <= 0 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 0`)
		}

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
		nodePoolBlockBody.SetAttributeValue("control_plane", cty.BoolVal(pool.Cp))
		nodePoolBlockBody.SetAttributeValue("etcd", cty.BoolVal(pool.Etcd))
		nodePoolBlockBody.SetAttributeValue("worker", cty.BoolVal(pool.Wkr))
		rootBody.AppendNewline()

		if num != len(nodePools) {
			clusterSyncNodePoolIDs = clusterSyncNodePoolIDs + `rancher2_node_pool.pool` + poolNum + `.id, `
		}
		if num == len(nodePools) {
			clusterSyncNodePoolIDs = clusterSyncNodePoolIDs + `rancher2_node_pool.pool` + poolNum + `.id`
		}

		num++
	}

	// Resource cluster sync
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

	// Write hcl file
	file.Write(f.Bytes())

	if err != nil {
		return false, err
	}
	return true, nil
}
