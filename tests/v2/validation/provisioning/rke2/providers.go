package rke2

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
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CloudCredFunc func(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error)
type MachinePoolFunc func(generatedPoolName, namespace string) *unstructured.Unstructured

type Provider struct {
	Name                               provisioning.ProviderName
	MachineConfigPoolResourceSteveType string
	MachinePoolFunc                    MachinePoolFunc
	CloudCredFunc                      CloudCredFunc
}

// CreateProvider returns all machine and cloud credential
// configs in the form of a Provider struct. Accepts a
// string of the name of the provider.
func CreateProvider(name string) Provider {
	switch {
	case name == provisioning.AWSProviderName.String():
		provider := Provider{
			Name:                               provisioning.AWSProviderName,
			MachineConfigPoolResourceSteveType: machinepools.AWSPoolType,
			MachinePoolFunc:                    machinepools.NewAWSMachineConfig,
			CloudCredFunc:                      aws.CreateAWSCloudCredentials,
		}
		return provider
	case name == provisioning.AzureProviderName.String():
		provider := Provider{
			Name:                               provisioning.AzureProviderName,
			MachineConfigPoolResourceSteveType: machinepools.AzurePoolType,
			MachinePoolFunc:                    machinepools.NewAzureMachineConfig,
			CloudCredFunc:                      azure.CreateAzureCloudCredentials,
		}
		return provider
	case name == provisioning.DOProviderName.String():
		provider := Provider{
			Name:                               provisioning.DOProviderName,
			MachineConfigPoolResourceSteveType: machinepools.DOPoolType,
			MachinePoolFunc:                    machinepools.NewDigitalOceanMachineConfig,
			CloudCredFunc:                      digitalocean.CreateDigitalOceanCloudCredentials,
		}
		return provider
	case name == provisioning.LinodeProviderName.String():
		provider := Provider{
			Name:                               provisioning.LinodeProviderName,
			MachineConfigPoolResourceSteveType: machinepools.LinodePoolType,
			MachinePoolFunc:                    machinepools.NewLinodeMachineConfig,
			CloudCredFunc:                      linode.CreateLinodeCloudCredentials,
		}
		return provider
	case name == provisioning.HarvesterProviderName.String():
		provider := Provider{
			Name:                               provisioning.HarvesterProviderName,
			MachineConfigPoolResourceSteveType: machinepools.HarvesterPoolType,
			MachinePoolFunc:                    machinepools.NewHarvesterMachineConfig,
			CloudCredFunc:                      harvester.CreateHarvesterCloudCredentials,
		}
		return provider
	case name == provisioning.VsphereProviderName.String():
		provider := Provider{
			Name:                               provisioning.VsphereProviderName,
			MachineConfigPoolResourceSteveType: machinepools.VmwarevsphereType,
			MachinePoolFunc:                    machinepools.NewVSphereMachineConfig,
			CloudCredFunc:                      vsphere.CreateVsphereCloudCredentials,
		}
		return provider
	default:
		panic(fmt.Sprintf("Provider:%v not found", name))
	}
}
