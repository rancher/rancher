package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
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
	AppRevisionsGetter
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

type ServiceAccountTokensGetter interface {
	ServiceAccountTokens(namespace string) ServiceAccountTokenInterface
}

func (c *Client) ServiceAccountTokens(namespace string) ServiceAccountTokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(ServiceAccountTokenGroupVersionResource, ServiceAccountTokenGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ServiceAccountTokenResource, ServiceAccountTokenGroupVersionKind, serviceAccountTokenFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(DockerCredentialGroupVersionResource, DockerCredentialGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &DockerCredentialResource, DockerCredentialGroupVersionKind, dockerCredentialFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(CertificateGroupVersionResource, CertificateGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &CertificateResource, CertificateGroupVersionKind, certificateFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(BasicAuthGroupVersionResource, BasicAuthGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &BasicAuthResource, BasicAuthGroupVersionKind, basicAuthFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(SSHAuthGroupVersionResource, SSHAuthGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SSHAuthResource, SSHAuthGroupVersionKind, sshAuthFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(NamespacedServiceAccountTokenGroupVersionResource, NamespacedServiceAccountTokenGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NamespacedServiceAccountTokenResource, NamespacedServiceAccountTokenGroupVersionKind, namespacedServiceAccountTokenFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(NamespacedDockerCredentialGroupVersionResource, NamespacedDockerCredentialGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NamespacedDockerCredentialResource, NamespacedDockerCredentialGroupVersionKind, namespacedDockerCredentialFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(NamespacedCertificateGroupVersionResource, NamespacedCertificateGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NamespacedCertificateResource, NamespacedCertificateGroupVersionKind, namespacedCertificateFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(NamespacedBasicAuthGroupVersionResource, NamespacedBasicAuthGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NamespacedBasicAuthResource, NamespacedBasicAuthGroupVersionKind, namespacedBasicAuthFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(NamespacedSSHAuthGroupVersionResource, NamespacedSSHAuthGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NamespacedSSHAuthResource, NamespacedSSHAuthGroupVersionKind, namespacedSshAuthFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(WorkloadGroupVersionResource, WorkloadGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &WorkloadResource, WorkloadGroupVersionKind, workloadFactory{})
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
	sharedClient := c.clientFactory.ForResourceKind(AppGroupVersionResource, AppGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &AppResource, AppGroupVersionKind, appFactory{})
	return &appClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type AppRevisionsGetter interface {
	AppRevisions(namespace string) AppRevisionInterface
}

func (c *Client) AppRevisions(namespace string) AppRevisionInterface {
	sharedClient := c.clientFactory.ForResourceKind(AppRevisionGroupVersionResource, AppRevisionGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &AppRevisionResource, AppRevisionGroupVersionKind, appRevisionFactory{})
	return &appRevisionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
