package nodetemplates

import (
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	"github.com/rancher/shepherd/extensions/cloudcredentials/aws"
	"github.com/rancher/shepherd/pkg/config"
)

const awsEC2NodeTemplateNameBase = "awsNodeConfig"
const providerName = "aws"

// CreateAWSNodeTemplate is a helper function that takes the rancher Client as a parameter and creates
// an AWS node template and returns the NodeTemplate response
func CreateAWSNodeTemplate(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error) {
	var amazonEC2NodeTemplateConfig nodetemplates.AmazonEC2NodeTemplateConfig
	config.LoadConfig(nodetemplates.AmazonEC2NodeTemplateConfigurationFileKey, &amazonEC2NodeTemplateConfig)

	cloudCredentialConfig := cloudcredentials.LoadCloudCredential(providerName)
	cloudCredential, err := aws.CreateAWSCloudCredentials(rancherClient, cloudCredentialConfig)
	if err != nil {
		return nil, err
	}

	nodeTemplate := nodetemplates.NodeTemplate{
		EngineInstallURL:            "https://releases.rancher.com/install-docker/24.0.sh",
		Name:                        awsEC2NodeTemplateNameBase,
		AmazonEC2NodeTemplateConfig: &amazonEC2NodeTemplateConfig,
	}

	nodeTemplateConfig := &nodetemplates.NodeTemplate{
		CloudCredentialID: cloudCredential.Namespace + ":" + cloudCredential.Name,
	}

	config.LoadConfig(nodetemplates.NodeTemplateConfigurationFileKey, nodeTemplateConfig)

	nodeTemplateFinal, err := nodeTemplate.MergeOverride(nodeTemplateConfig, nodetemplates.AmazonEC2NodeTemplateConfigurationFileKey)
	if err != nil {
		return nil, err
	}

	resp := &nodetemplates.NodeTemplate{}
	err = rancherClient.Management.APIBaseClient.Ops.DoCreate(management.NodeTemplateType, *nodeTemplateFinal, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
