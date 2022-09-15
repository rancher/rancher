package linode

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const linodeCloudCredNameBase = "linodeCloudCredential"

// CreateLinodeCloudCredentials is a helper function that takes the rancher Client as a parameter and creates
// a Linode cloud credential, and returns the CloudCredential response
func CreateLinodeCloudCredentials(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error) {
	var linodeCredentialConfig cloudcredentials.LinodeCredentialConfig
	config.LoadConfig(cloudcredentials.LinodeCredentialConfigurationFileKey, &linodeCredentialConfig)

	cloudCredential := cloudcredentials.CloudCredential{
		Name:                   linodeCloudCredNameBase,
		LinodeCredentialConfig: &linodeCredentialConfig,
	}

	resp := &cloudcredentials.CloudCredential{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.CloudCredentialType, cloudCredential, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
