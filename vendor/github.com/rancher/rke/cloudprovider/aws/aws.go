package aws

import (
	"bytes"
	"fmt"

	"github.com/go-ini/ini"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	AWSCloudProviderName = "aws"
	AWSConfig            = "AWSConfig"
)

type CloudProvider struct {
	Config *v3.AWSCloudProvider
	Name   string
}

func GetInstance() *CloudProvider {
	return &CloudProvider{}
}

func (p *CloudProvider) Init(cloudProviderConfig v3.CloudProvider) error {
	p.Name = AWSCloudProviderName
	if cloudProviderConfig.AWSCloudProvider == nil {
		return nil
	}
	p.Config = cloudProviderConfig.AWSCloudProvider

	return nil
}
func (p *CloudProvider) GetName() string {
	return p.Name
}

func (p *CloudProvider) GenerateCloudConfigFile() (string, error) {
	if p.Config == nil {
		return "", nil
	}
	// Generate INI style configuration
	buf := new(bytes.Buffer)
	cloudConfig, _ := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, []byte(""))
	if err := ini.ReflectFrom(cloudConfig, p.Config); err != nil {
		return "", fmt.Errorf("Failed to parse AWS cloud config")
	}
	if _, err := cloudConfig.WriteTo(buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
