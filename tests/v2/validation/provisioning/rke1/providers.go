package rke1

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	aws "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/aws"
	azure "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/azure"
	harvester "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/harvester"
	linode "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/linode"
)

const (
	awsProviderName       = "aws"
	azureProviderName     = "azure"
	harvesterProviderName = "harvester"
	linodeProviderName    = "linode"
)

type NodeTemplateFunc func(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error)

type Provider struct {
	Name             string
	NodeTemplateFunc NodeTemplateFunc
}

// CreateProvider returns all node template
// configs in the form of a Provider struct. Accepts a
// string of the name of the provider.
func CreateProvider(name string) Provider {
	switch {
	case name == awsProviderName:
		provider := Provider{
			Name:             name,
			NodeTemplateFunc: aws.CreateAWSNodeTemplate,
		}
		return provider
	case name == azureProviderName:
		provider := Provider{
			Name:             name,
			NodeTemplateFunc: azure.CreateAzureNodeTemplate,
		}
		return provider
	case name == harvesterProviderName:
		provider := Provider{
			Name:             name,
			NodeTemplateFunc: harvester.CreateHarvesterNodeTemplate,
		}
		return provider
	case name == linodeProviderName:
		provider := Provider{
			Name:             name,
			NodeTemplateFunc: linode.CreateLinodeNodeTemplate,
		}
		return provider
	default:
		panic(fmt.Sprintf("Provider:%v not found", name))
	}
}
