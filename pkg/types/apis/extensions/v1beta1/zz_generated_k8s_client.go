package v1beta1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	IngressesGetter
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

type IngressesGetter interface {
	Ingresses(namespace string) IngressInterface
}

func (c *Client) Ingresses(namespace string) IngressInterface {
	sharedClient := c.clientFactory.ForResourceKind(IngressGroupVersionResource, IngressGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &IngressResource, IngressGroupVersionKind, ingressFactory{})
	return &ingressClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
