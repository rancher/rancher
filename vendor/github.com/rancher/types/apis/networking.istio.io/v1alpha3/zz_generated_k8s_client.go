package v1alpha3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	VirtualServicesGetter
	DestinationRulesGetter
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

type VirtualServicesGetter interface {
	VirtualServices(namespace string) VirtualServiceInterface
}

func (c *Client) VirtualServices(namespace string) VirtualServiceInterface {
	sharedClient := c.clientFactory.ForResourceKind(VirtualServiceGroupVersionResource, VirtualServiceGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &VirtualServiceResource, VirtualServiceGroupVersionKind, virtualServiceFactory{})
	return &virtualServiceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type DestinationRulesGetter interface {
	DestinationRules(namespace string) DestinationRuleInterface
}

func (c *Client) DestinationRules(namespace string) DestinationRuleInterface {
	sharedClient := c.clientFactory.ForResourceKind(DestinationRuleGroupVersionResource, DestinationRuleGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &DestinationRuleResource, DestinationRuleGroupVersionKind, destinationRuleFactory{})
	return &destinationRuleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
