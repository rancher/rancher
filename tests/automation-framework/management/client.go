package management

import (
	managementClient "github.com/rancher/rancher/tests/automation-framework/client/generated/management/v3"
	"github.com/rancher/rancher/tests/automation-framework/clientbase"
	"github.com/rancher/rancher/tests/automation-framework/cloudcredentials"
	"github.com/rancher/rancher/tests/automation-framework/testsession"
)

type Client struct {
	*managementClient.Client
	CloudCredentialUpdated cloudcredentials.CloudCredentialOperations
}

func NewClient(opts *clientbase.ClientOpts, testSession *testsession.TestSession) (*Client, error) {
	client, err := managementClient.NewClient(opts)
	if err != nil {
		return nil, err
	}

	client.APIBaseClient.Ops.TestSession = testSession

	cloudClient, err := cloudcredentials.NewClient(opts, testSession)
	if err != nil {
		return nil, err
	}

	managementClient := &Client{
		client,
		cloudClient,
	}

	return managementClient, nil
}
