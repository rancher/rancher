package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	DeploymentsGetter
	DaemonSetsGetter
	StatefulSetsGetter
	ReplicaSetsGetter
}

type Client struct {
	controllerFactory controller.SharedControllerFactory
	clientFactory     client.SharedClientFactory
}

func NewForConfig(cfg rest.Config) (Interface, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
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

type DeploymentsGetter interface {
	Deployments(namespace string) DeploymentInterface
}

func (c *Client) Deployments(namespace string) DeploymentInterface {
	sharedClient := c.clientFactory.ForResourceKind(DeploymentGroupVersionResource, DeploymentGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &DeploymentResource, DeploymentGroupVersionKind, deploymentFactory{})
	return &deploymentClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type DaemonSetsGetter interface {
	DaemonSets(namespace string) DaemonSetInterface
}

func (c *Client) DaemonSets(namespace string) DaemonSetInterface {
	sharedClient := c.clientFactory.ForResourceKind(DaemonSetGroupVersionResource, DaemonSetGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &DaemonSetResource, DaemonSetGroupVersionKind, daemonSetFactory{})
	return &daemonSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type StatefulSetsGetter interface {
	StatefulSets(namespace string) StatefulSetInterface
}

func (c *Client) StatefulSets(namespace string) StatefulSetInterface {
	sharedClient := c.clientFactory.ForResourceKind(StatefulSetGroupVersionResource, StatefulSetGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &StatefulSetResource, StatefulSetGroupVersionKind, statefulSetFactory{})
	return &statefulSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ReplicaSetsGetter interface {
	ReplicaSets(namespace string) ReplicaSetInterface
}

func (c *Client) ReplicaSets(namespace string) ReplicaSetInterface {
	sharedClient := c.clientFactory.ForResourceKind(ReplicaSetGroupVersionResource, ReplicaSetGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ReplicaSetResource, ReplicaSetGroupVersionKind, replicaSetFactory{})
	return &replicaSetClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
