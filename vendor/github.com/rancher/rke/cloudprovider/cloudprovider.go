package cloudprovider

import (
	"github.com/rancher/rke/cloudprovider/aws"
	"github.com/rancher/rke/cloudprovider/azure"
	"github.com/rancher/rke/cloudprovider/custom"
	"github.com/rancher/rke/cloudprovider/openstack"
	"github.com/rancher/rke/cloudprovider/vsphere"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type CloudProvider interface {
	Init(cloudProviderConfig v3.CloudProvider) error
	GenerateCloudConfigFile() (string, error)
	GetName() string
}

func InitCloudProvider(cloudProviderConfig v3.CloudProvider) (CloudProvider, error) {
	var p CloudProvider
	if cloudProviderConfig.AWSCloudProvider != nil || cloudProviderConfig.Name == aws.AWSCloudProviderName {
		p = aws.GetInstance()
	}
	if cloudProviderConfig.AzureCloudProvider != nil || cloudProviderConfig.Name == azure.AzureCloudProviderName {
		p = azure.GetInstance()
	}
	if cloudProviderConfig.OpenstackCloudProvider != nil || cloudProviderConfig.Name == openstack.OpenstackCloudProviderName {
		p = openstack.GetInstance()
	}
	if cloudProviderConfig.VsphereCloudProvider != nil || cloudProviderConfig.Name == vsphere.VsphereCloudProviderName {
		p = vsphere.GetInstance()
	}
	if cloudProviderConfig.CustomCloudProvider != "" {
		p = custom.GetInstance()
	}

	if p != nil {
		if err := p.Init(cloudProviderConfig); err != nil {
			return nil, err
		}
	}
	return p, nil
}
