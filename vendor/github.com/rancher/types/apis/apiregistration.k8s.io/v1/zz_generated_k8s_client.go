package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	APIServicesGetter
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

type APIServicesGetter interface {
	APIServices(namespace string) APIServiceInterface
}

func (c *Client) APIServices(namespace string) APIServiceInterface {
	sharedClient := c.clientFactory.ForResourceKind(APIServiceGroupVersionResource, APIServiceGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &APIServiceResource, APIServiceGroupVersionKind, apiServiceFactory{})
	return &apiServiceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
