package nodetemplates

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const vmwarevsphereNodeTemplateNameBase = "vmwarevsphereNodeConfig"

// CreateVSphereNodeTemplate is a helper function that takes the rancher Client as a parameter and creates
// an VSphere node template and returns the NodeTemplate response
func CreateVSphereNodeTemplate(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error) {
	var vmwarevsphereNodeTemplateConfig nodetemplates.VmwareVsphereNodeTemplateConfig
	config.LoadConfig(nodetemplates.VmwareVsphereNodeTemplateConfigurationFileKey, &vmwarevsphereNodeTemplateConfig)

	nodeTemplate := nodetemplates.NodeTemplate{
		EngineInstallURL:                "https://releases.rancher.com/install-docker/20.10.sh",
		Name:                            vmwarevsphereNodeTemplateNameBase,
		VmwareVsphereNodeTemplateConfig: &vmwarevsphereNodeTemplateConfig,
	}

	nodeTemplateConfig := &nodetemplates.NodeTemplate{}
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
