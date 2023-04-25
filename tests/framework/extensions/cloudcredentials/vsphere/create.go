package vsphere

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const vmwarevsphereCloudCredNameBase = "vmwarevsphereCloudCredential"

// CreateVsphereCloudCredentials is a helper function that takes the rancher Client as a parameter and creates
// an AWS cloud credential, and returns the CloudCredential response
func CreateVsphereCloudCredentials(rancherClient *rancher.Client) (*cloudcredentials.CloudCredential, error) {
	var vmwarevsphereCredentialConfig cloudcredentials.VmwarevsphereCredentialConfig
	config.LoadConfig(cloudcredentials.VmwarevsphereCredentialConfigurationFileKey, &vmwarevsphereCredentialConfig)

	cloudCredential := cloudcredentials.CloudCredential{
		Name:                vmwarevsphereCloudCredNameBase,
		VmwareVsphereConfig: &vmwarevsphereCredentialConfig,
	}

	resp := &cloudcredentials.CloudCredential{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.CloudCredentialType, cloudCredential, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
