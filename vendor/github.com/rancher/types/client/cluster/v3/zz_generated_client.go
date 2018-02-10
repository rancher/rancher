package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	Namespace        NamespaceOperations
	Node             NodeOperations
	PersistentVolume PersistentVolumeOperations
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}

	client.Namespace = newNamespaceClient(client)
	client.Node = newNodeClient(client)
	client.PersistentVolume = newPersistentVolumeClient(client)

	return client, nil
}
