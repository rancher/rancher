package google

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const googleCloudCredNameBase = "googleCloudCredNameBase"

// CreateGoogleCloudCredentials is a helper function that takes the rancher Client as a parameter and creates
// a Google cloud credential, and returns the CloudCredential response
func CreateGoogleCloudCredentials(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error) {
	var googleCredentialConfig cloudcredentials.GoogleCredentialConfig
	config.LoadConfig(cloudcredentials.GoogleCredentialConfigurationFileKey, &googleCredentialConfig)

	cloudCredential := cloudcredentials.CloudCredential{
		Name:                   googleCloudCredNameBase,
		GoogleCredentialConfig: &googleCredentialConfig,
	}

	resp := &cloudcredentials.CloudCredential{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.CloudCredentialType, cloudCredential, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
