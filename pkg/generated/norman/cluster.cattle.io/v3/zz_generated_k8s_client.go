package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	ClusterAuthTokensGetter
	ClusterUserAttributesGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v3.AddToScheme(scheme); err != nil {
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

type ClusterAuthTokensGetter interface {
	ClusterAuthTokens(namespace string) ClusterAuthTokenInterface
}

func (c *Client) ClusterAuthTokens(namespace string) ClusterAuthTokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterAuthTokenGroupVersionResource, ClusterAuthTokenGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterAuthTokenResource, ClusterAuthTokenGroupVersionKind, clusterAuthTokenFactory{})
	return &clusterAuthTokenClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterUserAttributesGetter interface {
	ClusterUserAttributes(namespace string) ClusterUserAttributeInterface
}

func (c *Client) ClusterUserAttributes(namespace string) ClusterUserAttributeInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterUserAttributeGroupVersionResource, ClusterUserAttributeGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterUserAttributeResource, ClusterUserAttributeGroupVersionKind, clusterUserAttributeFactory{})
	return &clusterUserAttributeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
