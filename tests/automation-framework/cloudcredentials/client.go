package cloudcredentials

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	apiClient *clientbase.APIBaseClient
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		apiClient: &baseClient,
	}

	return client, nil
}
