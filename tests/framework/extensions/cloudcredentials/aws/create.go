package aws

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const awsCloudCredNameBase = "awsCloudCredential"

// CreateAWSCloudCredentials is a helper function that takes the rancher Client as a prameter and creates
// an AWS cloud credential, and returns the CloudCredential response
func CreateAWSCloudCredentials(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error) {
	var amazonEC2CredentialConfig cloudcredentials.AmazonEC2CredentialConfig
	config.LoadConfig(cloudcredentials.AmazonEC2CredentialConfigurationFileKey, &amazonEC2CredentialConfig)

	cloudCredential := cloudcredentials.CloudCredential{
		Name:                      awsCloudCredNameBase,
		AmazonEC2CredentialConfig: &amazonEC2CredentialConfig,
	}

	resp := &cloudcredentials.CloudCredential{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.CloudCredentialType, cloudCredential, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
