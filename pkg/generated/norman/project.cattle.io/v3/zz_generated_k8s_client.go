package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
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
	SourceCodeProvidersGetter
	SourceCodeProviderConfigsGetter
	SourceCodeCredentialsGetter
	PipelinesGetter
	PipelineExecutionsGetter
	PipelineSettingsGetter
	SourceCodeRepositoriesGetter
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

type SourceCodeProvidersGetter interface {
	SourceCodeProviders(namespace string) SourceCodeProviderInterface
}

func (c *Client) SourceCodeProviders(namespace string) SourceCodeProviderInterface {
	sharedClient := c.clientFactory.ForResourceKind(SourceCodeProviderGroupVersionResource, SourceCodeProviderGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SourceCodeProviderResource, SourceCodeProviderGroupVersionKind, sourceCodeProviderFactory{})
	return &sourceCodeProviderClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SourceCodeProviderConfigsGetter interface {
	SourceCodeProviderConfigs(namespace string) SourceCodeProviderConfigInterface
}

func (c *Client) SourceCodeProviderConfigs(namespace string) SourceCodeProviderConfigInterface {
	sharedClient := c.clientFactory.ForResourceKind(SourceCodeProviderConfigGroupVersionResource, SourceCodeProviderConfigGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SourceCodeProviderConfigResource, SourceCodeProviderConfigGroupVersionKind, sourceCodeProviderConfigFactory{})
	return &sourceCodeProviderConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SourceCodeCredentialsGetter interface {
	SourceCodeCredentials(namespace string) SourceCodeCredentialInterface
}

func (c *Client) SourceCodeCredentials(namespace string) SourceCodeCredentialInterface {
	sharedClient := c.clientFactory.ForResourceKind(SourceCodeCredentialGroupVersionResource, SourceCodeCredentialGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SourceCodeCredentialResource, SourceCodeCredentialGroupVersionKind, sourceCodeCredentialFactory{})
	return &sourceCodeCredentialClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PipelinesGetter interface {
	Pipelines(namespace string) PipelineInterface
}

func (c *Client) Pipelines(namespace string) PipelineInterface {
	sharedClient := c.clientFactory.ForResourceKind(PipelineGroupVersionResource, PipelineGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PipelineResource, PipelineGroupVersionKind, pipelineFactory{})
	return &pipelineClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PipelineExecutionsGetter interface {
	PipelineExecutions(namespace string) PipelineExecutionInterface
}

func (c *Client) PipelineExecutions(namespace string) PipelineExecutionInterface {
	sharedClient := c.clientFactory.ForResourceKind(PipelineExecutionGroupVersionResource, PipelineExecutionGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PipelineExecutionResource, PipelineExecutionGroupVersionKind, pipelineExecutionFactory{})
	return &pipelineExecutionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PipelineSettingsGetter interface {
	PipelineSettings(namespace string) PipelineSettingInterface
}

func (c *Client) PipelineSettings(namespace string) PipelineSettingInterface {
	sharedClient := c.clientFactory.ForResourceKind(PipelineSettingGroupVersionResource, PipelineSettingGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PipelineSettingResource, PipelineSettingGroupVersionKind, pipelineSettingFactory{})
	return &pipelineSettingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SourceCodeRepositoriesGetter interface {
	SourceCodeRepositories(namespace string) SourceCodeRepositoryInterface
}

func (c *Client) SourceCodeRepositories(namespace string) SourceCodeRepositoryInterface {
	sharedClient := c.clientFactory.ForResourceKind(SourceCodeRepositoryGroupVersionResource, SourceCodeRepositoryGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SourceCodeRepositoryResource, SourceCodeRepositoryGroupVersionKind, sourceCodeRepositoryFactory{})
	return &sourceCodeRepositoryClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
