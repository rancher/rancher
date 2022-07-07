package rke1

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	aws "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/aws"
	azure "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/azure"
	linode "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/linode"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	awsProviderName    = "aws"
	azureProviderName  = "azure"
	linodeProviderName = "linode"
)

type NodeTemplateFunc func(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error)
type MachinePoolFunc func(generatedPoolName, namespace string) *unstructured.Unstructured

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
