package rke1

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	aws "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/aws"
	azure "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/azure"
	harvester "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/harvester"
	linode "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/linode"
	vsphere "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/vsphere"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
)

type NodeTemplateFunc func(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error)

type Provider struct {
	Name             provisioning.ProviderName
	NodeTemplateFunc NodeTemplateFunc
}

// CreateProvider returns all node template
// configs in the form of a Provider struct. Accepts a
// string of the name of the provider.
func CreateProvider(name string) Provider {
	switch {
	case name == provisioning.AWSProviderName.String():
		provider := Provider{
			Name:             provisioning.AWSProviderName,
			NodeTemplateFunc: aws.CreateAWSNodeTemplate,
		}
		return provider
	case name == provisioning.AzureProviderName.String():
		provider := Provider{
			Name:             provisioning.AzureProviderName,
			NodeTemplateFunc: azure.CreateAzureNodeTemplate,
		}
		return provider
	case name == provisioning.HarvesterProviderName.String():
		provider := Provider{
			Name:             provisioning.HarvesterProviderName,
			NodeTemplateFunc: harvester.CreateHarvesterNodeTemplate,
		}
		return provider
	case name == provisioning.LinodeProviderName.String():
		provider := Provider{
			Name:             provisioning.LinodeProviderName,
			NodeTemplateFunc: linode.CreateLinodeNodeTemplate,
		}
		return provider
	case name == provisioning.VsphereProviderName.String():
		provider := Provider{
			Name:             provisioning.VsphereProviderName,
			NodeTemplateFunc: vsphere.CreateVSphereNodeTemplate,
		}
		return provider
	default:
		panic(fmt.Sprintf("Provider:%v not found", name))
	}
}
