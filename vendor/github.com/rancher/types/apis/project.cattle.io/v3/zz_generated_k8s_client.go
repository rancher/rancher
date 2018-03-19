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
	NamespacedServiceAccountTokensGetter
	NamespacedDockerCredentialsGetter
	NamespacedCertificatesGetter
	NamespacedBasicAuthsGetter
	NamespacedSSHAuthsGetter
	WorkloadsGetter
	AppsGetter
	NamespaceComposeConfigsGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	serviceAccountTokenControllers           map[string]ServiceAccountTokenController
	dockerCredentialControllers              map[string]DockerCredentialController
	certificateControllers                   map[string]CertificateController
	basicAuthControllers                     map[string]BasicAuthController
	sshAuthControllers                       map[string]SSHAuthController
	namespacedServiceAccountTokenControllers map[string]NamespacedServiceAccountTokenController
	namespacedDockerCredentialControllers    map[string]NamespacedDockerCredentialController
	namespacedCertificateControllers         map[string]NamespacedCertificateController
	namespacedBasicAuthControllers           map[string]NamespacedBasicAuthController
	namespacedSshAuthControllers             map[string]NamespacedSSHAuthController
	workloadControllers                      map[string]WorkloadController
	appControllers                           map[string]AppController
	namespaceComposeConfigControllers        map[string]NamespaceComposeConfigController
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

		serviceAccountTokenControllers:           map[string]ServiceAccountTokenController{},
		dockerCredentialControllers:              map[string]DockerCredentialController{},
		certificateControllers:                   map[string]CertificateController{},
		basicAuthControllers:                     map[string]BasicAuthController{},
		sshAuthControllers:                       map[string]SSHAuthController{},
		namespacedServiceAccountTokenControllers: map[string]NamespacedServiceAccountTokenController{},
		namespacedDockerCredentialControllers:    map[string]NamespacedDockerCredentialController{},
		namespacedCertificateControllers:         map[string]NamespacedCertificateController{},
		namespacedBasicAuthControllers:           map[string]NamespacedBasicAuthController{},
		namespacedSshAuthControllers:             map[string]NamespacedSSHAuthController{},
		workloadControllers:                      map[string]WorkloadController{},
		appControllers:                           map[string]AppController{},
		namespaceComposeConfigControllers:        map[string]NamespaceComposeConfigController{},
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

type NamespacedServiceAccountTokensGetter interface {
	NamespacedServiceAccountTokens(namespace string) NamespacedServiceAccountTokenInterface
}

func (c *Client) NamespacedServiceAccountTokens(namespace string) NamespacedServiceAccountTokenInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespacedServiceAccountTokenResource, NamespacedServiceAccountTokenGroupVersionKind, namespacedServiceAccountTokenFactory{})
	return &namespacedServiceAccountTokenClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespacedDockerCredentialsGetter interface {
	NamespacedDockerCredentials(namespace string) NamespacedDockerCredentialInterface
}

func (c *Client) NamespacedDockerCredentials(namespace string) NamespacedDockerCredentialInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespacedDockerCredentialResource, NamespacedDockerCredentialGroupVersionKind, namespacedDockerCredentialFactory{})
	return &namespacedDockerCredentialClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespacedCertificatesGetter interface {
	NamespacedCertificates(namespace string) NamespacedCertificateInterface
}

func (c *Client) NamespacedCertificates(namespace string) NamespacedCertificateInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespacedCertificateResource, NamespacedCertificateGroupVersionKind, namespacedCertificateFactory{})
	return &namespacedCertificateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespacedBasicAuthsGetter interface {
	NamespacedBasicAuths(namespace string) NamespacedBasicAuthInterface
}

func (c *Client) NamespacedBasicAuths(namespace string) NamespacedBasicAuthInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespacedBasicAuthResource, NamespacedBasicAuthGroupVersionKind, namespacedBasicAuthFactory{})
	return &namespacedBasicAuthClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespacedSSHAuthsGetter interface {
	NamespacedSSHAuths(namespace string) NamespacedSSHAuthInterface
}

func (c *Client) NamespacedSSHAuths(namespace string) NamespacedSSHAuthInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespacedSSHAuthResource, NamespacedSSHAuthGroupVersionKind, namespacedSshAuthFactory{})
	return &namespacedSshAuthClient{
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

type AppsGetter interface {
	Apps(namespace string) AppInterface
}

func (c *Client) Apps(namespace string) AppInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &AppResource, AppGroupVersionKind, appFactory{})
	return &appClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NamespaceComposeConfigsGetter interface {
	NamespaceComposeConfigs(namespace string) NamespaceComposeConfigInterface
}

func (c *Client) NamespaceComposeConfigs(namespace string) NamespaceComposeConfigInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &NamespaceComposeConfigResource, NamespaceComposeConfigGroupVersionKind, namespaceComposeConfigFactory{})
	return &namespaceComposeConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
