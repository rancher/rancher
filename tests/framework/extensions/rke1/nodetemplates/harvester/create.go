package nodetemplates

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const harvesterNodeTemplateNameBase = "harvesterNodeConfig"

// CreateHarvesterNodeTemplate is a helper function that takes the rancher Client as a parameter and creates
// an Harvester node template and returns the NodeTemplate response
func CreateHarvesterNodeTemplate(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error) {
	var harvesterNodeTemplateConfig nodetemplates.HarvesterNodeTemplateConfig
	config.LoadConfig(nodetemplates.HarvesterNodeTemplateConfigurationFileKey, &harvesterNodeTemplateConfig)

	nodeTemplate := nodetemplates.NodeTemplate{
		EngineInstallURL:            "https://releases.rancher.com/install-docker/24.0.sh",
		Name:                        harvesterNodeTemplateNameBase,
		HarvesterNodeTemplateConfig: &harvesterNodeTemplateConfig,
	}

	resp := &nodetemplates.NodeTemplate{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.NodeTemplateType, nodeTemplate, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
