package vmwarevsphere

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const vSphereCloudCredNameBase = "vSphereCloudCredential"

// CreateVSphereCloudCredentials is a helper function that takes the rancher Client as a parameter and creates
// a vSphere cloud credential and returns the CloudCredential response
func CreateVSphereCloudCredentials(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error) {
	var vsphereCredentialConfig cloudcredentials.VSphereCredentialConfig
	config.LoadConfig(cloudcredentials.VSphereCredentialConfigurationFileKey, &vsphereCredentialConfig)

	cloudCredential := cloudcredentials.CloudCredential{
		Name:                    vSphereCloudCredNameBase,
		VSphereCredentialConfig: &vsphereCredentialConfig,
	}

	resp := &cloudcredentials.CloudCredential{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.CloudCredentialType, cloudCredential, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
