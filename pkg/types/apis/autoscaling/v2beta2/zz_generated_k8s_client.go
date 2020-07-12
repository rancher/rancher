package v2beta2

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	HorizontalPodAutoscalersGetter
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

type HorizontalPodAutoscalersGetter interface {
	HorizontalPodAutoscalers(namespace string) HorizontalPodAutoscalerInterface
}

func (c *Client) HorizontalPodAutoscalers(namespace string) HorizontalPodAutoscalerInterface {
	sharedClient := c.clientFactory.ForResourceKind(HorizontalPodAutoscalerGroupVersionResource, HorizontalPodAutoscalerGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &HorizontalPodAutoscalerResource, HorizontalPodAutoscalerGroupVersionKind, horizontalPodAutoscalerFactory{})
	return &horizontalPodAutoscalerClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
