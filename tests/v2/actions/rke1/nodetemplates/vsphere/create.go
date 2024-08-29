package nodetemplates

import (
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	"github.com/rancher/shepherd/extensions/cloudcredentials/vsphere"
	"github.com/rancher/shepherd/pkg/config"
)

const vmwarevsphereNodeTemplateNameBase = "vmwarevsphereNodeConfig"
const providerName = "vsphere"

// CreateVSphereNodeTemplate is a helper function that takes the rancher Client as a parameter and creates
// a VSphere node template and returns the NodeTemplate response
func CreateVSphereNodeTemplate(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error) {
	var vmwarevsphereNodeTemplateConfig nodetemplates.VmwareVsphereNodeTemplateConfig
	config.LoadConfig(nodetemplates.VmwareVsphereNodeTemplateConfigurationFileKey, &vmwarevsphereNodeTemplateConfig)

	cloudCredentialConfig := cloudcredentials.LoadCloudCredential(providerName)
	cloudCredential, err := vsphere.CreateVsphereCloudCredentials(rancherClient, cloudCredentialConfig)
	if err != nil {
		return nil, err
	}

	nodeTemplate := nodetemplates.NodeTemplate{
		EngineInstallURL:                "https://releases.rancher.com/install-docker/20.10.sh",
		Name:                            vmwarevsphereNodeTemplateNameBase,
		VmwareVsphereNodeTemplateConfig: &vmwarevsphereNodeTemplateConfig,
	}

	nodeTemplateConfig := &nodetemplates.NodeTemplate{
		CloudCredentialID: cloudCredential.Namespace + ":" + cloudCredential.Name,
	}

	config.LoadConfig(nodetemplates.NodeTemplateConfigurationFileKey, nodeTemplateConfig)

	nodeTemplateFinal, err := nodeTemplate.
		MergeOverride(nodeTemplateConfig, nodetemplates.VmwareVsphereNodeTemplateConfigurationFileKey)
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

// GetVsphereNodeTemplate is a helper to get the vsphere node template from a config
func GetVsphereNodeTemplate() *nodetemplates.VmwareVsphereNodeTemplateConfig {
	var vmwarevsphereNodeTemplateConfig nodetemplates.VmwareVsphereNodeTemplateConfig
	config.LoadConfig(nodetemplates.VmwareVsphereNodeTemplateConfigurationFileKey, &vmwarevsphereNodeTemplateConfig)

	return &vmwarevsphereNodeTemplateConfig
}
