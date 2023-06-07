package functions

import (
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/v2/validation/terratest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func SetGKE(t *testing.T, k8sVersion string, nodePools []terratest.Nodepool, file *os.File) (done bool, err error) {
	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(terratest.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	googleAuthEncodedJSONConfig := new(terratest.GoogleAuthEncodedJSON)
	config.LoadConfig("googleAuthEncodedJSON", googleAuthEncodedJSONConfig)

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

	googleCredConfigBlock := cloudCredBlockBody.AppendNewBlock("google_credential_config", nil)

	authEncodedJSON := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(`jsonencode({"type" = "` + googleAuthEncodedJSONConfig.Type + `", "project_id" = "` + googleAuthEncodedJSONConfig.ProjectID + `", "private_key_id" = "` + googleAuthEncodedJSONConfig.PrivateKeyID + `", "private_key" = "` + googleAuthEncodedJSONConfig.PrivateKey + `", "client_email" = "` + googleAuthEncodedJSONConfig.ClientEmail + `", "client_id" = "` + googleAuthEncodedJSONConfig.ClientID + `", "auth_uri" = "` + googleAuthEncodedJSONConfig.AuthURI + `", "token_uri" = "` + googleAuthEncodedJSONConfig.TokenURI + `", "auth_provider_x509_cert_url" = "` + googleAuthEncodedJSONConfig.AuthProviderX509CertURL + `", "client_x509_cert_url" = "` + googleAuthEncodedJSONConfig.ClientX509CertURL + `"})`)},
	}

	googleCredConfigBlock.Body().SetAttributeRaw("auth_encoded_json", authEncodedJSON)

	rootBody.AppendNewline()

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

	num := 1
	for _, pool := range nodePools {
		poolNum := strconv.Itoa(num)

		result, err := SetResourceNodepoolValidation(t, pool, poolNum)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		nodePoolsBlock := gkeConfigBlockBody.AppendNewBlock("node_pools", nil)
		nodePoolsBlockBody := nodePoolsBlock.Body()

		nodePoolsBlockBody.SetAttributeValue("initial_node_count", cty.NumberIntVal(pool.Quantity))
		nodePoolsBlockBody.SetAttributeValue("max_pods_constraint", cty.NumberIntVal(pool.MaxPodsContraint))
		nodePoolsBlockBody.SetAttributeValue("name", cty.StringVal(terraformConfig.HostnamePrefix+`-pool`+poolNum))
		nodePoolsBlockBody.SetAttributeValue("version", cty.StringVal(k8sVersion))

		num++
	}

	_, err = file.Write(f.Bytes())

	if err != nil {
		t.Logf("Failed to write GKE configurations to main.tf file. Error: %v", err)
		return false, err
	}

	return true, nil
}
