package azure

import (
	"encoding/json"
	"fmt"

	v3 "github.com/rancher/rke/types"
)

const (
	AzureCloudProviderName = "azure"
)

type CloudProvider struct {
	Config *v3.AzureCloudProvider
	Name   string
}

func GetInstance() *CloudProvider {
	return &CloudProvider{}
}

func (p *CloudProvider) Init(cloudProviderConfig v3.CloudProvider) error {
	if cloudProviderConfig.AzureCloudProvider == nil {
		return fmt.Errorf("Azure Cloud Provider Config is empty")
	}
	p.Name = AzureCloudProviderName
	if cloudProviderConfig.Name != "" {
		p.Name = cloudProviderConfig.Name
	}
	p.Config = cloudProviderConfig.AzureCloudProvider
	return nil
}

func (p *CloudProvider) GetName() string {
	return p.Name
}

func (p *CloudProvider) GenerateCloudConfigFile() (string, error) {
	cloudConfig, err := json.MarshalIndent(p.Config, "", "\n")
	if err != nil {
		return "", err
	}
	return string(cloudConfig), nil
}
