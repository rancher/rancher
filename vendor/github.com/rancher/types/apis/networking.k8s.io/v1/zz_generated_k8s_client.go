package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	NetworkPoliciesGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewFromControllerFactory(factory controller.SharedControllerFactory) (Interface, error) {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}, nil
}

type NetworkPoliciesGetter interface {
	NetworkPolicies(namespace string) NetworkPolicyInterface
}

func (c *Client) NetworkPolicies(namespace string) NetworkPolicyInterface {
	sharedClient := c.clientFactory.ForResourceKind(NetworkPolicyGroupVersionResource, NetworkPolicyGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NetworkPolicyResource, NetworkPolicyGroupVersionKind, networkPolicyFactory{})
	return &networkPolicyClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
