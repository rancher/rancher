package custom

import "github.com/rancher/types/apis/management.cattle.io/v3"

type CloudProvider struct {
	Name   string
	Config string
}

func GetInstance() *CloudProvider {
	return &CloudProvider{}
}

func (p *CloudProvider) Init(cloudProviderConfig v3.CloudProvider) error {
	p.Name = cloudProviderConfig.Name
	p.Config = cloudProviderConfig.CustomCloudProvider
	return nil
}

func (p *CloudProvider) GetName() string {
	return p.Name
}

func (p *CloudProvider) GenerateCloudConfigFile() (string, error) {
	return p.Config, nil
}
