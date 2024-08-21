package nodetemplates

import (
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/cloudcredentials/azure"
	"github.com/rancher/shepherd/pkg/config"
)

const azureNodeTemplateNameBase = "azureNodeConfig"

// CreateAzureNodeTemplate is a helper function that takes the rancher Client as a parameter and creates
// an Azure node template and returns the NodeTemplate response
func CreateAzureNodeTemplate(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error) {
	var azureNodeTemplateConfig nodetemplates.AzureNodeTemplateConfig
	config.LoadConfig(nodetemplates.AzureNodeTemplateConfigurationFileKey, &azureNodeTemplateConfig)

	cloudCredential, err := azure.CreateAzureCloudCredentials(rancherClient)
	if err != nil {
		return nil, err
	}

	nodeTemplate := nodetemplates.NodeTemplate{
		EngineInstallURL:        "https://releases.rancher.com/install-docker/23.0.sh",
		Name:                    azureNodeTemplateNameBase,
		AzureNodeTemplateConfig: &azureNodeTemplateConfig,
	}

	nodeTemplateConfig := &nodetemplates.NodeTemplate{
		CloudCredentialID: cloudCredential.ID,
	}

	config.LoadConfig(nodetemplates.NodeTemplateConfigurationFileKey, nodeTemplateConfig)

	nodeTemplateFinal, err := nodeTemplate.MergeOverride(nodeTemplateConfig, nodetemplates.AzureNodeTemplateConfigurationFileKey)
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
