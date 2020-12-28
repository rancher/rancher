package v2beta2

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	HorizontalPodAutoscalersGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v2beta2.AddToScheme(scheme); err != nil {
		return nil, err
	}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(&cfg, scheme)
	if err != nil {
		return nil, err
	}
	return NewFromControllerFactory(controllerFactory)
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
