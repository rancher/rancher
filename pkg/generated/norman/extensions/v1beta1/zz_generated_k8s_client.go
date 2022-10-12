package v1beta1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	IngressesGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	sharedOpts := &controller.SharedControllerFactoryOptions{
		SyncOnlyChangedObjects: generator.SyncOnlyChangedObjects(),
	}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(&cfg, scheme, sharedOpts)
	if err != nil {
		return nil, err
	}
	return NewFromControllerFactory(controllerFactory), nil
}

func NewFromControllerFactory(factory controller.SharedControllerFactory) Interface {
	return &Client{
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}
}

func NewFromControllerFactoryWithAgent(userAgent string, factory controller.SharedControllerFactory) Interface {
	return &Client{
		controllerFactory: factory,
		clientFactory:     client.NewSharedClientFactoryWithAgent(userAgent, factory.SharedCacheFactory().SharedClientFactory()),
	}
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
