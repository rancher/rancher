package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
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

type ServiceAccountTokensGetter interface {
	ServiceAccountTokens(namespace string) ServiceAccountTokenInterface
}

func (c *Client) ServiceAccountTokens(namespace string) ServiceAccountTokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(ServiceAccountTokenGroupVersionResource, ServiceAccountTokenGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ServiceAccountTokens] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ServiceAccountTokenResource, ServiceAccountTokenGroupVersionKind, serviceAccountTokenFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [DockerCredentials] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &DockerCredentialResource, DockerCredentialGroupVersionKind, dockerCredentialFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Certificates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CertificateResource, CertificateGroupVersionKind, certificateFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [BasicAuths] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &BasicAuthResource, BasicAuthGroupVersionKind, basicAuthFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [SSHAuths] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SSHAuthResource, SSHAuthGroupVersionKind, sshAuthFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NamespacedServiceAccountTokens] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NamespacedServiceAccountTokenResource, NamespacedServiceAccountTokenGroupVersionKind, namespacedServiceAccountTokenFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NamespacedDockerCredentials] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NamespacedDockerCredentialResource, NamespacedDockerCredentialGroupVersionKind, namespacedDockerCredentialFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NamespacedCertificates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NamespacedCertificateResource, NamespacedCertificateGroupVersionKind, namespacedCertificateFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NamespacedBasicAuths] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NamespacedBasicAuthResource, NamespacedBasicAuthGroupVersionKind, namespacedBasicAuthFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NamespacedSSHAuths] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NamespacedSSHAuthResource, NamespacedSSHAuthGroupVersionKind, namespacedSshAuthFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Workloads] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &WorkloadResource, WorkloadGroupVersionKind, workloadFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Apps] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &AppResource, AppGroupVersionKind, appFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [AppRevisions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &AppRevisionResource, AppRevisionGroupVersionKind, appRevisionFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [SourceCodeProviders] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SourceCodeProviderResource, SourceCodeProviderGroupVersionKind, sourceCodeProviderFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [SourceCodeProviderConfigs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SourceCodeProviderConfigResource, SourceCodeProviderConfigGroupVersionKind, sourceCodeProviderConfigFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [SourceCodeCredentials] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SourceCodeCredentialResource, SourceCodeCredentialGroupVersionKind, sourceCodeCredentialFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Pipelines] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PipelineResource, PipelineGroupVersionKind, pipelineFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [PipelineExecutions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PipelineExecutionResource, PipelineExecutionGroupVersionKind, pipelineExecutionFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [PipelineSettings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PipelineSettingResource, PipelineSettingGroupVersionKind, pipelineSettingFactory{})
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
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [SourceCodeRepositories] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SourceCodeRepositoryResource, SourceCodeRepositoryGroupVersionKind, sourceCodeRepositoryFactory{})
	return &sourceCodeRepositoryClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
