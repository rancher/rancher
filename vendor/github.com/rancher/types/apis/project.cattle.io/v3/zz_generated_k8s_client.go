package v3

import (
	"context"
	"sync"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	ServiceAccountTokensGetter
	DockerCredentialsGetter
	CertificatesGetter
	BasicAuthsGetter
	SSHAuthsGetter
	WorkloadsGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	serviceAccountTokenControllers map[string]ServiceAccountTokenController
	dockerCredentialControllers    map[string]DockerCredentialController
	certificateControllers         map[string]CertificateController
	basicAuthControllers           map[string]BasicAuthController
	sshAuthControllers             map[string]SSHAuthController
	workloadControllers            map[string]WorkloadController
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		config.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	restClient, err := rest.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		serviceAccountTokenControllers: map[string]ServiceAccountTokenController{},
		dockerCredentialControllers:    map[string]DockerCredentialController{},
		certificateControllers:         map[string]CertificateController{},
		basicAuthControllers:           map[string]BasicAuthController{},
		sshAuthControllers:             map[string]SSHAuthController{},
		workloadControllers:            map[string]WorkloadController{},
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

type ServiceAccountTokensGetter interface {
	ServiceAccountTokens(namespace string) ServiceAccountTokenInterface
}

func (c *Client) ServiceAccountTokens(namespace string) ServiceAccountTokenInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ServiceAccountTokenResource, ServiceAccountTokenGroupVersionKind, serviceAccountTokenFactory{})
	return &serviceAccountTokenClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type DockerCredentialsGetter interface {
	DockerCredentials(namespace string) DockerCredentialInterface
}

func (c *Client) DockerCredentials(namespace string) DockerCredentialInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &DockerCredentialResource, DockerCredentialGroupVersionKind, dockerCredentialFactory{})
	return &dockerCredentialClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CertificatesGetter interface {
	Certificates(namespace string) CertificateInterface
}

func (c *Client) Certificates(namespace string) CertificateInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &CertificateResource, CertificateGroupVersionKind, certificateFactory{})
	return &certificateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type BasicAuthsGetter interface {
	BasicAuths(namespace string) BasicAuthInterface
}

func (c *Client) BasicAuths(namespace string) BasicAuthInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &BasicAuthResource, BasicAuthGroupVersionKind, basicAuthFactory{})
	return &basicAuthClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SSHAuthsGetter interface {
	SSHAuths(namespace string) SSHAuthInterface
}

func (c *Client) SSHAuths(namespace string) SSHAuthInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &SSHAuthResource, SSHAuthGroupVersionKind, sshAuthFactory{})
	return &sshAuthClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type WorkloadsGetter interface {
	Workloads(namespace string) WorkloadInterface
}

func (c *Client) Workloads(namespace string) WorkloadInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &WorkloadResource, WorkloadGroupVersionKind, workloadFactory{})
	return &workloadClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
