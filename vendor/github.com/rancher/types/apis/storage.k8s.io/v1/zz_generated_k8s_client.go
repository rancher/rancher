package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
)

type Interface interface {
	StorageClassesGetter
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

type StorageClassesGetter interface {
	StorageClasses(namespace string) StorageClassInterface
}

func (c *Client) StorageClasses(namespace string) StorageClassInterface {
	sharedClient := c.clientFactory.ForResourceKind(StorageClassGroupVersionResource, StorageClassGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &StorageClassResource, StorageClassGroupVersionKind, storageClassFactory{})
	return &storageClassClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
