package v1

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	NodesGetter
	ComponentStatusesGetter
	NamespacesGetter
	EventsGetter
	EndpointsGetter
	PersistentVolumeClaimsGetter
	PodsGetter
	ServicesGetter
	SecretsGetter
	ConfigMapsGetter
	ServiceAccountsGetter
	ReplicationControllersGetter
	ResourceQuotasGetter
	LimitRangesGetter
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

type NodesGetter interface {
	Nodes(namespace string) NodeInterface
}

func (c *Client) Nodes(namespace string) NodeInterface {
	sharedClient := c.clientFactory.ForResourceKind(NodeGroupVersionResource, NodeGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NodeResource, NodeGroupVersionKind, nodeFactory{})
	return &nodeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ComponentStatusesGetter interface {
	ComponentStatuses(namespace string) ComponentStatusInterface
}

func (c *Client) ComponentStatuses(namespace string) ComponentStatusInterface {
	sharedClient := c.clientFactory.ForResourceKind(ComponentStatusGroupVersionResource, ComponentStatusGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ComponentStatusResource, ComponentStatusGroupVersionKind, componentStatusFactory{})
	return &componentStatusClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespacesGetter interface {
	Namespaces(namespace string) NamespaceInterface
}

func (c *Client) Namespaces(namespace string) NamespaceInterface {
	sharedClient := c.clientFactory.ForResourceKind(NamespaceGroupVersionResource, NamespaceGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NamespaceResource, NamespaceGroupVersionKind, namespaceFactory{})
	return &namespaceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type EventsGetter interface {
	Events(namespace string) EventInterface
}

func (c *Client) Events(namespace string) EventInterface {
	sharedClient := c.clientFactory.ForResourceKind(EventGroupVersionResource, EventGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &EventResource, EventGroupVersionKind, eventFactory{})
	return &eventClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type EndpointsGetter interface {
	Endpoints(namespace string) EndpointsInterface
}

func (c *Client) Endpoints(namespace string) EndpointsInterface {
	sharedClient := c.clientFactory.ForResourceKind(EndpointsGroupVersionResource, EndpointsGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &EndpointsResource, EndpointsGroupVersionKind, endpointsFactory{})
	return &endpointsClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PersistentVolumeClaimsGetter interface {
	PersistentVolumeClaims(namespace string) PersistentVolumeClaimInterface
}

func (c *Client) PersistentVolumeClaims(namespace string) PersistentVolumeClaimInterface {
	sharedClient := c.clientFactory.ForResourceKind(PersistentVolumeClaimGroupVersionResource, PersistentVolumeClaimGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PersistentVolumeClaimResource, PersistentVolumeClaimGroupVersionKind, persistentVolumeClaimFactory{})
	return &persistentVolumeClaimClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PodsGetter interface {
	Pods(namespace string) PodInterface
}

func (c *Client) Pods(namespace string) PodInterface {
	sharedClient := c.clientFactory.ForResourceKind(PodGroupVersionResource, PodGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PodResource, PodGroupVersionKind, podFactory{})
	return &podClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ServicesGetter interface {
	Services(namespace string) ServiceInterface
}

func (c *Client) Services(namespace string) ServiceInterface {
	sharedClient := c.clientFactory.ForResourceKind(ServiceGroupVersionResource, ServiceGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ServiceResource, ServiceGroupVersionKind, serviceFactory{})
	return &serviceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SecretsGetter interface {
	Secrets(namespace string) SecretInterface
}

func (c *Client) Secrets(namespace string) SecretInterface {
	sharedClient := c.clientFactory.ForResourceKind(SecretGroupVersionResource, SecretGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SecretResource, SecretGroupVersionKind, secretFactory{})
	return &secretClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ConfigMapsGetter interface {
	ConfigMaps(namespace string) ConfigMapInterface
}

func (c *Client) ConfigMaps(namespace string) ConfigMapInterface {
	sharedClient := c.clientFactory.ForResourceKind(ConfigMapGroupVersionResource, ConfigMapGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ConfigMapResource, ConfigMapGroupVersionKind, configMapFactory{})
	return &configMapClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ServiceAccountsGetter interface {
	ServiceAccounts(namespace string) ServiceAccountInterface
}

func (c *Client) ServiceAccounts(namespace string) ServiceAccountInterface {
	sharedClient := c.clientFactory.ForResourceKind(ServiceAccountGroupVersionResource, ServiceAccountGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ServiceAccountResource, ServiceAccountGroupVersionKind, serviceAccountFactory{})
	return &serviceAccountClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ReplicationControllersGetter interface {
	ReplicationControllers(namespace string) ReplicationControllerInterface
}

func (c *Client) ReplicationControllers(namespace string) ReplicationControllerInterface {
	sharedClient := c.clientFactory.ForResourceKind(ReplicationControllerGroupVersionResource, ReplicationControllerGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ReplicationControllerResource, ReplicationControllerGroupVersionKind, replicationControllerFactory{})
	return &replicationControllerClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ResourceQuotasGetter interface {
	ResourceQuotas(namespace string) ResourceQuotaInterface
}

func (c *Client) ResourceQuotas(namespace string) ResourceQuotaInterface {
	sharedClient := c.clientFactory.ForResourceKind(ResourceQuotaGroupVersionResource, ResourceQuotaGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ResourceQuotaResource, ResourceQuotaGroupVersionKind, resourceQuotaFactory{})
	return &resourceQuotaClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type LimitRangesGetter interface {
	LimitRanges(namespace string) LimitRangeInterface
}

func (c *Client) LimitRanges(namespace string) LimitRangeInterface {
	sharedClient := c.clientFactory.ForResourceKind(LimitRangeGroupVersionResource, LimitRangeGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &LimitRangeResource, LimitRangeGroupVersionKind, limitRangeFactory{})
	return &limitRangeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
