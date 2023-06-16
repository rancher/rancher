package provisioning

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/aws"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/azure"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/digitalocean"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/harvester"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/linode"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/vsphere"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	r1aws "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/aws"
	r1azure "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/azure"
	r1harvester "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/harvester"
	r1linode "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/linode"
	r1vsphere "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/vsphere"
)

type CloudCredFunc func(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error)
type MachinePoolFunc func(generatedPoolName, namespace string) *unstructured.Unstructured

type Provider struct {
	Name                               provisioninginput.ProviderName
	MachineConfigPoolResourceSteveType string
	MachinePoolFunc                    MachinePoolFunc
	CloudCredFunc                      CloudCredFunc
}

// CreateProvider returns all machine and cloud credential
// configs in the form of a Provider struct. Accepts a
// string of the name of the provider.
func CreateProvider(name string) Provider {
	switch {
	case name == provisioninginput.AWSProviderName.String():
		provider := Provider{
			Name:                               provisioninginput.AWSProviderName,
			MachineConfigPoolResourceSteveType: machinepools.AWSPoolType,
			MachinePoolFunc:                    machinepools.NewAWSMachineConfig,
			CloudCredFunc:                      aws.CreateAWSCloudCredentials,
		}
		return provider
	case name == provisioninginput.AzureProviderName.String():
		provider := Provider{
			Name:                               provisioninginput.AzureProviderName,
			MachineConfigPoolResourceSteveType: machinepools.AzurePoolType,
			MachinePoolFunc:                    machinepools.NewAzureMachineConfig,
			CloudCredFunc:                      azure.CreateAzureCloudCredentials,
		}
		return provider
	case name == provisioninginput.DOProviderName.String():
		provider := Provider{
			Name:                               provisioninginput.DOProviderName,
			MachineConfigPoolResourceSteveType: machinepools.DOPoolType,
			MachinePoolFunc:                    machinepools.NewDigitalOceanMachineConfig,
			CloudCredFunc:                      digitalocean.CreateDigitalOceanCloudCredentials,
		}
		return provider
	case name == provisioninginput.LinodeProviderName.String():
		provider := Provider{
			Name:                               provisioninginput.LinodeProviderName,
			MachineConfigPoolResourceSteveType: machinepools.LinodePoolType,
			MachinePoolFunc:                    machinepools.NewLinodeMachineConfig,
			CloudCredFunc:                      linode.CreateLinodeCloudCredentials,
		}
		return provider
	case name == provisioninginput.HarvesterProviderName.String():
		provider := Provider{
			Name:                               provisioninginput.HarvesterProviderName,
			MachineConfigPoolResourceSteveType: machinepools.HarvesterPoolType,
			MachinePoolFunc:                    machinepools.NewHarvesterMachineConfig,
			CloudCredFunc:                      harvester.CreateHarvesterCloudCredentials,
		}
		return provider
	case name == provisioninginput.VsphereProviderName.String():
		provider := Provider{
			Name:                               provisioninginput.VsphereProviderName,
			MachineConfigPoolResourceSteveType: machinepools.VmwarevsphereType,
			MachinePoolFunc:                    machinepools.NewVSphereMachineConfig,
			CloudCredFunc:                      vsphere.CreateVsphereCloudCredentials,
		}
		return provider
	default:
		panic(fmt.Sprintf("Provider:%v not found", name))
	}
}

type NodeTemplateFunc func(rancherClient *rancher.Client) (*nodetemplates.NodeTemplate, error)

type RKE1Provider struct {
	Name             provisioninginput.ProviderName
	NodeTemplateFunc NodeTemplateFunc
}

// CreateProvider returns all node template
// configs in the form of a RKE1Provider struct. Accepts a
// string of the name of the provider.
func CreateRKE1Provider(name string) RKE1Provider {
	switch {
	case name == provisioninginput.AWSProviderName.String():
		provider := RKE1Provider{
			Name:             provisioninginput.AWSProviderName,
			NodeTemplateFunc: r1aws.CreateAWSNodeTemplate,
		}
		return provider
	case name == provisioninginput.AzureProviderName.String():
		provider := RKE1Provider{
			Name:             provisioninginput.AzureProviderName,
			NodeTemplateFunc: r1azure.CreateAzureNodeTemplate,
		}
		return provider
	case name == provisioninginput.HarvesterProviderName.String():
		provider := RKE1Provider{
			Name:             provisioninginput.HarvesterProviderName,
			NodeTemplateFunc: r1harvester.CreateHarvesterNodeTemplate,
		}
		return provider
	case name == provisioninginput.LinodeProviderName.String():
		provider := RKE1Provider{
			Name:             provisioninginput.LinodeProviderName,
			NodeTemplateFunc: r1linode.CreateLinodeNodeTemplate,
		}
		return provider
	case name == provisioninginput.VsphereProviderName.String():
		provider := RKE1Provider{
			Name:             provisioninginput.VsphereProviderName,
			NodeTemplateFunc: r1vsphere.CreateVSphereNodeTemplate,
		}
		return provider
	default:
		panic(fmt.Sprintf("RKE1Provider:%v not found", name))
	}
}
