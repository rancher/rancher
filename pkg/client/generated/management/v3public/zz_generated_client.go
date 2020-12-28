package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	AuthToken    AuthTokenOperations
	AuthProvider AuthProviderOperations
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}

	client.AuthToken = newAuthTokenClient(client)
	client.AuthProvider = newAuthProviderClient(client)

	return client, nil
}
