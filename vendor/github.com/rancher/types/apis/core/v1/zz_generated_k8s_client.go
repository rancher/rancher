package v1

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/rest"
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

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
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	nodeControllers                  map[string]NodeController
	componentStatusControllers       map[string]ComponentStatusController
	namespaceControllers             map[string]NamespaceController
	eventControllers                 map[string]EventController
	endpointsControllers             map[string]EndpointsController
	persistentVolumeClaimControllers map[string]PersistentVolumeClaimController
	podControllers                   map[string]PodController
	serviceControllers               map[string]ServiceController
	secretControllers                map[string]SecretController
	configMapControllers             map[string]ConfigMapController
	serviceAccountControllers        map[string]ServiceAccountController
	replicationControllerControllers map[string]ReplicationControllerController
	resourceQuotaControllers         map[string]ResourceQuotaController
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	restClient, err := restwatch.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		nodeControllers:                  map[string]NodeController{},
		componentStatusControllers:       map[string]ComponentStatusController{},
		namespaceControllers:             map[string]NamespaceController{},
		eventControllers:                 map[string]EventController{},
		endpointsControllers:             map[string]EndpointsController{},
		persistentVolumeClaimControllers: map[string]PersistentVolumeClaimController{},
		podControllers:                   map[string]PodController{},
		serviceControllers:               map[string]ServiceController{},
		secretControllers:                map[string]SecretController{},
		configMapControllers:             map[string]ConfigMapController{},
		serviceAccountControllers:        map[string]ServiceAccountController{},
		replicationControllerControllers: map[string]ReplicationControllerController{},
		resourceQuotaControllers:         map[string]ResourceQuotaController{},
	}, nil
}

func (c *Client) RESTClient() rest.Interface {
	return c.restClient
}

func (c *Client) Sync(ctx context.Context) error {
	return controller.Sync(ctx, c.starters...)
}

func (c *Client) Start(ctx context.Context, threadiness int) error {
	return controller.Start(ctx, threadiness, c.starters...)
}

type NodesGetter interface {
	Nodes(namespace string) NodeInterface
}

func (c *Client) Nodes(namespace string) NodeInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NodeResource, NodeGroupVersionKind, nodeFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ComponentStatusResource, ComponentStatusGroupVersionKind, componentStatusFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NamespaceResource, NamespaceGroupVersionKind, namespaceFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &EventResource, EventGroupVersionKind, eventFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &EndpointsResource, EndpointsGroupVersionKind, endpointsFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PersistentVolumeClaimResource, PersistentVolumeClaimGroupVersionKind, persistentVolumeClaimFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PodResource, PodGroupVersionKind, podFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ServiceResource, ServiceGroupVersionKind, serviceFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &SecretResource, SecretGroupVersionKind, secretFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ConfigMapResource, ConfigMapGroupVersionKind, configMapFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ServiceAccountResource, ServiceAccountGroupVersionKind, serviceAccountFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ReplicationControllerResource, ReplicationControllerGroupVersionKind, replicationControllerFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ResourceQuotaResource, ResourceQuotaGroupVersionKind, resourceQuotaFactory{})
	return &resourceQuotaClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
