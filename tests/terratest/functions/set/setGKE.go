package functions

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/terratest/tests"
	"github.com/zclconf/go-cty/cty"
)

func SetGKE(k8sVersion string, nodePools []tests.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(tests.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	googleAuthEncodedJSONConfig := new(tests.GoogleAuthEncodedJSON)
	config.LoadConfig("googleAuthEncodedJSON", googleAuthEncodedJSONConfig)

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
	googleCredConfigBlock := cloudCredBlockBody.AppendNewBlock("google_credential_config", nil)
	authEncodedJSON := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`jsonencode({"type" = "` + googleAuthEncodedJSONConfig.Type + `", "project_id" = "` + googleAuthEncodedJSONConfig.ProjectID + `", "private_key_id" = "` + googleAuthEncodedJSONConfig.PrivateKeyID + `", "private_key" = "` + googleAuthEncodedJSONConfig.PrivateKey + `", "client_email" = "` + googleAuthEncodedJSONConfig.ClientEmail + `", "client_id" = "` + googleAuthEncodedJSONConfig.ClientID + `", "auth_uri" = "` + googleAuthEncodedJSONConfig.AuthURI + `", "token_uri" = "` + googleAuthEncodedJSONConfig.TokenURI + `", "auth_provider_x509_cert_url" = "` + googleAuthEncodedJSONConfig.AuthProviderX509CertURL + `", "client_x509_cert_url" = "` + googleAuthEncodedJSONConfig.ClientX509CertURL + `"})`)},
	}
	googleCredConfigBlock.Body().SetAttributeRaw("auth_encoded_json", authEncodedJSON)
	rootBody.AppendNewline()

	// Resource cluster
	clusterBlock := rootBody.AppendNewBlock("resource", []string{"rancher2_cluster", "rancher2_cluster"})
	clusterBlockBody := clusterBlock.Body()
	clusterBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.ClusterName))
	gkeConfigBlock := clusterBlockBody.AppendNewBlock("gke_config_v2", nil)
	gkeConfigBlockBody := gkeConfigBlock.Body()
	gkeConfigBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.ClusterName))
	cloudCredSecret := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`rancher2_cloud_credential.rancher2_cloud_credential.id`)},
	}
	gkeConfigBlockBody.SetAttributeRaw("google_credential_secret", cloudCredSecret)
	gkeConfigBlockBody.SetAttributeValue("region", cty.StringVal(terraformConfig.Region))
	gkeConfigBlockBody.SetAttributeValue("project_id", cty.StringVal(terraformConfig.GKEProjectID))
	gkeConfigBlockBody.SetAttributeValue("kubernetes_version", cty.StringVal(k8sVersion))
	gkeConfigBlockBody.SetAttributeValue("network", cty.StringVal(terraformConfig.GKENetwork))
	gkeConfigBlockBody.SetAttributeValue("subnetwork", cty.StringVal(terraformConfig.GKESubnetwork))

	// Resource node pools
	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)
		if pool.Quantity <= 0 {
			return false, fmt.Errorf(`invalid quantity specified for pool` + poolNum + `; quantity must be greater than 0`)
		}
		nodePoolsBlock := gkeConfigBlockBody.AppendNewBlock("node_pools", nil)
		nodePoolsBlockBody := nodePoolsBlock.Body()
		nodePoolsBlockBody.SetAttributeValue("initial_node_count", cty.NumberIntVal(pool.Quantity))
		nodePoolsBlockBody.SetAttributeValue("max_pods_constraint", cty.NumberIntVal(pool.MaxPodsContraint))
		nodePoolsBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.HostnamePrefix+`-pool`+poolNum))
		nodePoolsBlockBody.SetAttributeValue("version", cty.StringVal(k8sVersion))

		num++
	}

	// Write hcl file
	file.Write(f.Bytes())

	if err != nil {
		return false, err
	}
	return true, nil
}
