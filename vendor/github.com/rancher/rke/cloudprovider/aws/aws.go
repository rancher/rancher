package aws

import "github.com/rancher/types/apis/management.cattle.io/v3"

type CloudProvider struct {
	Name string
}

const (
	AWSCloudProviderName = "aws"
)

func GetInstance() *CloudProvider {
	return &CloudProvider{}
}

func (p *CloudProvider) Init(cloudProviderConfig v3.CloudProvider) error {
	p.Name = AWSCloudProviderName
	return nil
}

func (p *CloudProvider) GetName() string {
	return p.Name
}

func (p *CloudProvider) GenerateCloudConfigFile() (string, error) {
	return "", nil
}
