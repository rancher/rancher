package nodetemplates

import (
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	"github.com/rancher/shepherd/extensions/cloudcredentials/linode"
	"github.com/rancher/shepherd/pkg/config"
)

const linodeNodeTemplateNameBase = "linodeNodeConfig"
const providerName = "linode"

// CreateLinodeNodeTemplate is a helper function that takes the rancher Client as a parameter and creates
// an Linode node template and returns the NodeTemplate response
func CreateLinodeNodeTemplate(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error) {
	var linodeNodeTemplateConfig nodetemplates.LinodeNodeTemplateConfig
	config.LoadConfig(nodetemplates.LinodeNodeTemplateConfigurationFileKey, &linodeNodeTemplateConfig)

	cloudCredentialConfig := cloudcredentials.LoadCloudCredential(providerName)
	cloudCredential, err := linode.CreateLinodeCloudCredentials(rancherClient, cloudCredentialConfig)
	if err != nil {
		return nil, err
	}

	nodeTemplate := nodetemplates.NodeTemplate{
		EngineInstallURL:         "https://releases.rancher.com/install-docker/24.0.sh",
		Name:                     linodeNodeTemplateNameBase,
		LinodeNodeTemplateConfig: &linodeNodeTemplateConfig,
	}

	nodeTemplateConfig := &nodetemplates.NodeTemplate{
		CloudCredentialID: cloudCredential.Namespace + ":" + cloudCredential.Name,
	}

	config.LoadConfig(nodetemplates.NodeTemplateConfigurationFileKey, nodeTemplateConfig)

	nodeTemplateFinal, err := nodeTemplate.MergeOverride(nodeTemplateConfig, nodetemplates.LinodeNodeTemplateConfigurationFileKey)
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
