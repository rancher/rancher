package vsphere

import (
	"fmt"

	"github.com/rancher/rke/templates"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	VsphereCloudProviderName = "vsphere"
	VsphereConfig            = "VsphereConfig"
)

type CloudProvider struct {
	Config *v3.VsphereCloudProvider
	Name   string
}

func GetInstance() *CloudProvider {
	return &CloudProvider{}
}

func (p *CloudProvider) Init(cloudProviderConfig v3.CloudProvider) error {
	if cloudProviderConfig.VsphereCloudProvider == nil {
		return fmt.Errorf("Vsphere Cloud Provider Config is empty")
	}
	p.Name = VsphereCloudProviderName
	if cloudProviderConfig.Name != "" {
		p.Name = cloudProviderConfig.Name
	}
	p.Config = cloudProviderConfig.VsphereCloudProvider
	return nil
}

func (p *CloudProvider) GetName() string {
	return p.Name
}

func (p *CloudProvider) GenerateCloudConfigFile() (string, error) {
	// Generate INI style configuration from template https://github.com/go-ini/ini/issues/84
	VsphereConfig := map[string]v3.VsphereCloudProvider{
		VsphereConfig: *p.Config,
	}
	return templates.CompileTemplateFromMap(templates.VsphereCloudProviderTemplate, VsphereConfig)
}
