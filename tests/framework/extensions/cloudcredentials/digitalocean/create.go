package digitalocean

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const digitalOceanCloudCredNameBase = "digitalOceanCloudCredential"

func CreateDigitalOceanCloudCredentials(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error) {
	var digitalOceanCredentialConfig cloudcredentials.DigitalOceanCredentialConfig
	config.LoadConfig(cloudcredentials.DigitalOceanCredentialConfigurationFileKey, &digitalOceanCredentialConfig)

	cloudCredential := cloudcredentials.CloudCredential{
		Name:                         digitalOceanCloudCredNameBase,
		DigitalOceanCredentialConfig: &digitalOceanCredentialConfig,
	}

	resp := &cloudcredentials.CloudCredential{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.CloudCredentialType, cloudCredential, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
