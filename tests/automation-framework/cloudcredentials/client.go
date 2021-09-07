package cloudcredentials

import (
	"github.com/rancher/rancher/tests/automation-framework/clientbase"
	"github.com/rancher/rancher/tests/automation-framework/testsession"
)

type Client struct {
	apiClient *clientbase.APIBaseClient
}

func NewClient(opts *clientbase.ClientOpts, testSession *testsession.TestSession) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		apiClient: &baseClient,
	}
	client.apiClient.Ops.TestSession = testSession

	return client, nil
}
