package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	ClusterAuthTokensGetter
	ClusterUserAttributesGetter
}

type Client struct {
	userAgent         string
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v3.AddToScheme(scheme); err != nil {
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
		userAgent:         userAgent,
		controllerFactory: factory,
		clientFactory:     factory.SharedCacheFactory().SharedClientFactory(),
	}
}

type ClusterAuthTokensGetter interface {
	ClusterAuthTokens(namespace string) ClusterAuthTokenInterface
}

func (c *Client) ClusterAuthTokens(namespace string) ClusterAuthTokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterAuthTokenGroupVersionResource, ClusterAuthTokenGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterAuthTokens] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterAuthTokenResource, ClusterAuthTokenGroupVersionKind, clusterAuthTokenFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterUserAttributes] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterUserAttributeResource, ClusterUserAttributeGroupVersionKind, clusterUserAttributeFactory{})
	return &clusterUserAttributeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
