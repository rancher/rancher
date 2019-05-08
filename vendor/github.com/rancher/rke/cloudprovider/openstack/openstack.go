package openstack

import (
	"bytes"
	"fmt"

	"github.com/go-ini/ini"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	OpenstackCloudProviderName = "openstack"
)

type CloudProvider struct {
	Config *v3.OpenstackCloudProvider
	Name   string
}

func GetInstance() *CloudProvider {
	return &CloudProvider{}
}

func (p *CloudProvider) Init(cloudProviderConfig v3.CloudProvider) error {
	if cloudProviderConfig.OpenstackCloudProvider == nil {
		return fmt.Errorf("Openstack Cloud Provider Config is empty")
	}
	p.Name = OpenstackCloudProviderName
	if cloudProviderConfig.Name != "" {
		p.Name = cloudProviderConfig.Name
	}
	p.Config = cloudProviderConfig.OpenstackCloudProvider
	return nil
}

func (p *CloudProvider) GetName() string {
	return p.Name
}

func (p *CloudProvider) GenerateCloudConfigFile() (string, error) {
	// Generate INI style configuration
	buf := new(bytes.Buffer)
	cloudConfig, _ := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, []byte(""))
	if err := ini.ReflectFrom(cloudConfig, p.Config); err != nil {
		return "", fmt.Errorf("Failed to parse Openstack cloud config")
	}
	if _, err := cloudConfig.WriteTo(buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
