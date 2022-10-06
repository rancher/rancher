package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	NodePoolsGetter
	NodesGetter
	NodeDriversGetter
	NodeTemplatesGetter
	ProjectsGetter
	GlobalRolesGetter
	GlobalRoleBindingsGetter
	RoleTemplatesGetter
	PodSecurityPolicyTemplatesGetter
	PodSecurityPolicyTemplateProjectBindingsGetter
	ClusterRoleTemplateBindingsGetter
	ProjectRoleTemplateBindingsGetter
	ClustersGetter
	ClusterRegistrationTokensGetter
	CatalogsGetter
	TemplatesGetter
	CatalogTemplatesGetter
	CatalogTemplateVersionsGetter
	TemplateVersionsGetter
	TemplateContentsGetter
	GroupsGetter
	GroupMembersGetter
	SamlTokensGetter
	PrincipalsGetter
	UsersGetter
	AuthConfigsGetter
	LdapConfigsGetter
	TokensGetter
	DynamicSchemasGetter
	PreferencesGetter
	UserAttributesGetter
	ProjectNetworkPoliciesGetter
	ClusterLoggingsGetter
	ProjectLoggingsGetter
	SettingsGetter
	FeaturesGetter
	ClusterAlertsGetter
	ProjectAlertsGetter
	NotifiersGetter
	ClusterAlertGroupsGetter
	ProjectAlertGroupsGetter
	ClusterAlertRulesGetter
	ProjectAlertRulesGetter
	ComposeConfigsGetter
	ProjectCatalogsGetter
	ClusterCatalogsGetter
	MultiClusterAppsGetter
	MultiClusterAppRevisionsGetter
	GlobalDnsesGetter
	GlobalDnsProvidersGetter
	KontainerDriversGetter
	EtcdBackupsGetter
	ClusterScansGetter
	MonitorMetricsGetter
	ClusterMonitorGraphsGetter
	ProjectMonitorGraphsGetter
	CloudCredentialsGetter
	ClusterTemplatesGetter
	ClusterTemplateRevisionsGetter
	RkeK8sSystemImagesGetter
	RkeK8sServiceOptionsGetter
	RkeAddonsGetter
	CisConfigsGetter
	CisBenchmarkVersionsGetter
	FleetWorkspacesGetter
	RancherUserNotificationsGetter
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

type NodePoolsGetter interface {
	NodePools(namespace string) NodePoolInterface
}

func (c *Client) NodePools(namespace string) NodePoolInterface {
	sharedClient := c.clientFactory.ForResourceKind(NodePoolGroupVersionResource, NodePoolGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NodePools] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NodePoolResource, NodePoolGroupVersionKind, nodePoolFactory{})
	return &nodePoolClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NodesGetter interface {
	Nodes(namespace string) NodeInterface
}

func (c *Client) Nodes(namespace string) NodeInterface {
	sharedClient := c.clientFactory.ForResourceKind(NodeGroupVersionResource, NodeGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Nodes] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NodeResource, NodeGroupVersionKind, nodeFactory{})
	return &nodeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NodeDriversGetter interface {
	NodeDrivers(namespace string) NodeDriverInterface
}

func (c *Client) NodeDrivers(namespace string) NodeDriverInterface {
	sharedClient := c.clientFactory.ForResourceKind(NodeDriverGroupVersionResource, NodeDriverGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NodeDrivers] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NodeDriverResource, NodeDriverGroupVersionKind, nodeDriverFactory{})
	return &nodeDriverClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NodeTemplatesGetter interface {
	NodeTemplates(namespace string) NodeTemplateInterface
}

func (c *Client) NodeTemplates(namespace string) NodeTemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(NodeTemplateGroupVersionResource, NodeTemplateGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [NodeTemplates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NodeTemplateResource, NodeTemplateGroupVersionKind, nodeTemplateFactory{})
	return &nodeTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectsGetter interface {
	Projects(namespace string) ProjectInterface
}

func (c *Client) Projects(namespace string) ProjectInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectGroupVersionResource, ProjectGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Projects] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectResource, ProjectGroupVersionKind, projectFactory{})
	return &projectClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GlobalRolesGetter interface {
	GlobalRoles(namespace string) GlobalRoleInterface
}

func (c *Client) GlobalRoles(namespace string) GlobalRoleInterface {
	sharedClient := c.clientFactory.ForResourceKind(GlobalRoleGroupVersionResource, GlobalRoleGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [GlobalRoles] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &GlobalRoleResource, GlobalRoleGroupVersionKind, globalRoleFactory{})
	return &globalRoleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GlobalRoleBindingsGetter interface {
	GlobalRoleBindings(namespace string) GlobalRoleBindingInterface
}

func (c *Client) GlobalRoleBindings(namespace string) GlobalRoleBindingInterface {
	sharedClient := c.clientFactory.ForResourceKind(GlobalRoleBindingGroupVersionResource, GlobalRoleBindingGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [GlobalRoleBindings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &GlobalRoleBindingResource, GlobalRoleBindingGroupVersionKind, globalRoleBindingFactory{})
	return &globalRoleBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RoleTemplatesGetter interface {
	RoleTemplates(namespace string) RoleTemplateInterface
}

func (c *Client) RoleTemplates(namespace string) RoleTemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(RoleTemplateGroupVersionResource, RoleTemplateGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [RoleTemplates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RoleTemplateResource, RoleTemplateGroupVersionKind, roleTemplateFactory{})
	return &roleTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PodSecurityPolicyTemplatesGetter interface {
	PodSecurityPolicyTemplates(namespace string) PodSecurityPolicyTemplateInterface
}

func (c *Client) PodSecurityPolicyTemplates(namespace string) PodSecurityPolicyTemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(PodSecurityPolicyTemplateGroupVersionResource, PodSecurityPolicyTemplateGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [PodSecurityPolicyTemplates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PodSecurityPolicyTemplateResource, PodSecurityPolicyTemplateGroupVersionKind, podSecurityPolicyTemplateFactory{})
	return &podSecurityPolicyTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PodSecurityPolicyTemplateProjectBindingsGetter interface {
	PodSecurityPolicyTemplateProjectBindings(namespace string) PodSecurityPolicyTemplateProjectBindingInterface
}

func (c *Client) PodSecurityPolicyTemplateProjectBindings(namespace string) PodSecurityPolicyTemplateProjectBindingInterface {
	sharedClient := c.clientFactory.ForResourceKind(PodSecurityPolicyTemplateProjectBindingGroupVersionResource, PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [PodSecurityPolicyTemplateProjectBindings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PodSecurityPolicyTemplateProjectBindingResource, PodSecurityPolicyTemplateProjectBindingGroupVersionKind, podSecurityPolicyTemplateProjectBindingFactory{})
	return &podSecurityPolicyTemplateProjectBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterRoleTemplateBindingsGetter interface {
	ClusterRoleTemplateBindings(namespace string) ClusterRoleTemplateBindingInterface
}

func (c *Client) ClusterRoleTemplateBindings(namespace string) ClusterRoleTemplateBindingInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterRoleTemplateBindingGroupVersionResource, ClusterRoleTemplateBindingGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterRoleTemplateBindings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterRoleTemplateBindingResource, ClusterRoleTemplateBindingGroupVersionKind, clusterRoleTemplateBindingFactory{})
	return &clusterRoleTemplateBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectRoleTemplateBindingsGetter interface {
	ProjectRoleTemplateBindings(namespace string) ProjectRoleTemplateBindingInterface
}

func (c *Client) ProjectRoleTemplateBindings(namespace string) ProjectRoleTemplateBindingInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectRoleTemplateBindingGroupVersionResource, ProjectRoleTemplateBindingGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectRoleTemplateBindings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectRoleTemplateBindingResource, ProjectRoleTemplateBindingGroupVersionKind, projectRoleTemplateBindingFactory{})
	return &projectRoleTemplateBindingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClustersGetter interface {
	Clusters(namespace string) ClusterInterface
}

func (c *Client) Clusters(namespace string) ClusterInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterGroupVersionResource, ClusterGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Clusters] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterResource, ClusterGroupVersionKind, clusterFactory{})
	return &clusterClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterRegistrationTokensGetter interface {
	ClusterRegistrationTokens(namespace string) ClusterRegistrationTokenInterface
}

func (c *Client) ClusterRegistrationTokens(namespace string) ClusterRegistrationTokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterRegistrationTokenGroupVersionResource, ClusterRegistrationTokenGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterRegistrationTokens] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterRegistrationTokenResource, ClusterRegistrationTokenGroupVersionKind, clusterRegistrationTokenFactory{})
	return &clusterRegistrationTokenClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CatalogsGetter interface {
	Catalogs(namespace string) CatalogInterface
}

func (c *Client) Catalogs(namespace string) CatalogInterface {
	sharedClient := c.clientFactory.ForResourceKind(CatalogGroupVersionResource, CatalogGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Catalogs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CatalogResource, CatalogGroupVersionKind, catalogFactory{})
	return &catalogClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type TemplatesGetter interface {
	Templates(namespace string) TemplateInterface
}

func (c *Client) Templates(namespace string) TemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(TemplateGroupVersionResource, TemplateGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Templates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &TemplateResource, TemplateGroupVersionKind, templateFactory{})
	return &templateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CatalogTemplatesGetter interface {
	CatalogTemplates(namespace string) CatalogTemplateInterface
}

func (c *Client) CatalogTemplates(namespace string) CatalogTemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(CatalogTemplateGroupVersionResource, CatalogTemplateGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [CatalogTemplates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CatalogTemplateResource, CatalogTemplateGroupVersionKind, catalogTemplateFactory{})
	return &catalogTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CatalogTemplateVersionsGetter interface {
	CatalogTemplateVersions(namespace string) CatalogTemplateVersionInterface
}

func (c *Client) CatalogTemplateVersions(namespace string) CatalogTemplateVersionInterface {
	sharedClient := c.clientFactory.ForResourceKind(CatalogTemplateVersionGroupVersionResource, CatalogTemplateVersionGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [CatalogTemplateVersions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CatalogTemplateVersionResource, CatalogTemplateVersionGroupVersionKind, catalogTemplateVersionFactory{})
	return &catalogTemplateVersionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type TemplateVersionsGetter interface {
	TemplateVersions(namespace string) TemplateVersionInterface
}

func (c *Client) TemplateVersions(namespace string) TemplateVersionInterface {
	sharedClient := c.clientFactory.ForResourceKind(TemplateVersionGroupVersionResource, TemplateVersionGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [TemplateVersions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &TemplateVersionResource, TemplateVersionGroupVersionKind, templateVersionFactory{})
	return &templateVersionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type TemplateContentsGetter interface {
	TemplateContents(namespace string) TemplateContentInterface
}

func (c *Client) TemplateContents(namespace string) TemplateContentInterface {
	sharedClient := c.clientFactory.ForResourceKind(TemplateContentGroupVersionResource, TemplateContentGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [TemplateContents] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &TemplateContentResource, TemplateContentGroupVersionKind, templateContentFactory{})
	return &templateContentClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GroupsGetter interface {
	Groups(namespace string) GroupInterface
}

func (c *Client) Groups(namespace string) GroupInterface {
	sharedClient := c.clientFactory.ForResourceKind(GroupGroupVersionResource, GroupGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Groups] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &GroupResource, GroupGroupVersionKind, groupFactory{})
	return &groupClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GroupMembersGetter interface {
	GroupMembers(namespace string) GroupMemberInterface
}

func (c *Client) GroupMembers(namespace string) GroupMemberInterface {
	sharedClient := c.clientFactory.ForResourceKind(GroupMemberGroupVersionResource, GroupMemberGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [GroupMembers] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &GroupMemberResource, GroupMemberGroupVersionKind, groupMemberFactory{})
	return &groupMemberClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SamlTokensGetter interface {
	SamlTokens(namespace string) SamlTokenInterface
}

func (c *Client) SamlTokens(namespace string) SamlTokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(SamlTokenGroupVersionResource, SamlTokenGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [SamlTokens] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SamlTokenResource, SamlTokenGroupVersionKind, samlTokenFactory{})
	return &samlTokenClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PrincipalsGetter interface {
	Principals(namespace string) PrincipalInterface
}

func (c *Client) Principals(namespace string) PrincipalInterface {
	sharedClient := c.clientFactory.ForResourceKind(PrincipalGroupVersionResource, PrincipalGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Principals] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PrincipalResource, PrincipalGroupVersionKind, principalFactory{})
	return &principalClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type UsersGetter interface {
	Users(namespace string) UserInterface
}

func (c *Client) Users(namespace string) UserInterface {
	sharedClient := c.clientFactory.ForResourceKind(UserGroupVersionResource, UserGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Users] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &UserResource, UserGroupVersionKind, userFactory{})
	return &userClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type AuthConfigsGetter interface {
	AuthConfigs(namespace string) AuthConfigInterface
}

func (c *Client) AuthConfigs(namespace string) AuthConfigInterface {
	sharedClient := c.clientFactory.ForResourceKind(AuthConfigGroupVersionResource, AuthConfigGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [AuthConfigs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &AuthConfigResource, AuthConfigGroupVersionKind, authConfigFactory{})
	return &authConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type LdapConfigsGetter interface {
	LdapConfigs(namespace string) LdapConfigInterface
}

func (c *Client) LdapConfigs(namespace string) LdapConfigInterface {
	sharedClient := c.clientFactory.ForResourceKind(LdapConfigGroupVersionResource, LdapConfigGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [LdapConfigs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &LdapConfigResource, LdapConfigGroupVersionKind, ldapConfigFactory{})
	return &ldapConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type TokensGetter interface {
	Tokens(namespace string) TokenInterface
}

func (c *Client) Tokens(namespace string) TokenInterface {
	sharedClient := c.clientFactory.ForResourceKind(TokenGroupVersionResource, TokenGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Tokens] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &TokenResource, TokenGroupVersionKind, tokenFactory{})
	return &tokenClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type DynamicSchemasGetter interface {
	DynamicSchemas(namespace string) DynamicSchemaInterface
}

func (c *Client) DynamicSchemas(namespace string) DynamicSchemaInterface {
	sharedClient := c.clientFactory.ForResourceKind(DynamicSchemaGroupVersionResource, DynamicSchemaGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [DynamicSchemas] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &DynamicSchemaResource, DynamicSchemaGroupVersionKind, dynamicSchemaFactory{})
	return &dynamicSchemaClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PreferencesGetter interface {
	Preferences(namespace string) PreferenceInterface
}

func (c *Client) Preferences(namespace string) PreferenceInterface {
	sharedClient := c.clientFactory.ForResourceKind(PreferenceGroupVersionResource, PreferenceGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Preferences] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &PreferenceResource, PreferenceGroupVersionKind, preferenceFactory{})
	return &preferenceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type UserAttributesGetter interface {
	UserAttributes(namespace string) UserAttributeInterface
}

func (c *Client) UserAttributes(namespace string) UserAttributeInterface {
	sharedClient := c.clientFactory.ForResourceKind(UserAttributeGroupVersionResource, UserAttributeGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [UserAttributes] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &UserAttributeResource, UserAttributeGroupVersionKind, userAttributeFactory{})
	return &userAttributeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectNetworkPoliciesGetter interface {
	ProjectNetworkPolicies(namespace string) ProjectNetworkPolicyInterface
}

func (c *Client) ProjectNetworkPolicies(namespace string) ProjectNetworkPolicyInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectNetworkPolicyGroupVersionResource, ProjectNetworkPolicyGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectNetworkPolicies] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectNetworkPolicyResource, ProjectNetworkPolicyGroupVersionKind, projectNetworkPolicyFactory{})
	return &projectNetworkPolicyClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterLoggingsGetter interface {
	ClusterLoggings(namespace string) ClusterLoggingInterface
}

func (c *Client) ClusterLoggings(namespace string) ClusterLoggingInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterLoggingGroupVersionResource, ClusterLoggingGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterLoggings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterLoggingResource, ClusterLoggingGroupVersionKind, clusterLoggingFactory{})
	return &clusterLoggingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectLoggingsGetter interface {
	ProjectLoggings(namespace string) ProjectLoggingInterface
}

func (c *Client) ProjectLoggings(namespace string) ProjectLoggingInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectLoggingGroupVersionResource, ProjectLoggingGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectLoggings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectLoggingResource, ProjectLoggingGroupVersionKind, projectLoggingFactory{})
	return &projectLoggingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SettingsGetter interface {
	Settings(namespace string) SettingInterface
}

func (c *Client) Settings(namespace string) SettingInterface {
	sharedClient := c.clientFactory.ForResourceKind(SettingGroupVersionResource, SettingGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Settings] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &SettingResource, SettingGroupVersionKind, settingFactory{})
	return &settingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type FeaturesGetter interface {
	Features(namespace string) FeatureInterface
}

func (c *Client) Features(namespace string) FeatureInterface {
	sharedClient := c.clientFactory.ForResourceKind(FeatureGroupVersionResource, FeatureGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Features] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &FeatureResource, FeatureGroupVersionKind, featureFactory{})
	return &featureClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterAlertsGetter interface {
	ClusterAlerts(namespace string) ClusterAlertInterface
}

func (c *Client) ClusterAlerts(namespace string) ClusterAlertInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterAlertGroupVersionResource, ClusterAlertGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterAlerts] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterAlertResource, ClusterAlertGroupVersionKind, clusterAlertFactory{})
	return &clusterAlertClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectAlertsGetter interface {
	ProjectAlerts(namespace string) ProjectAlertInterface
}

func (c *Client) ProjectAlerts(namespace string) ProjectAlertInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectAlertGroupVersionResource, ProjectAlertGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectAlerts] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectAlertResource, ProjectAlertGroupVersionKind, projectAlertFactory{})
	return &projectAlertClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type NotifiersGetter interface {
	Notifiers(namespace string) NotifierInterface
}

func (c *Client) Notifiers(namespace string) NotifierInterface {
	sharedClient := c.clientFactory.ForResourceKind(NotifierGroupVersionResource, NotifierGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [Notifiers] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &NotifierResource, NotifierGroupVersionKind, notifierFactory{})
	return &notifierClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterAlertGroupsGetter interface {
	ClusterAlertGroups(namespace string) ClusterAlertGroupInterface
}

func (c *Client) ClusterAlertGroups(namespace string) ClusterAlertGroupInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterAlertGroupGroupVersionResource, ClusterAlertGroupGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterAlertGroups] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterAlertGroupResource, ClusterAlertGroupGroupVersionKind, clusterAlertGroupFactory{})
	return &clusterAlertGroupClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectAlertGroupsGetter interface {
	ProjectAlertGroups(namespace string) ProjectAlertGroupInterface
}

func (c *Client) ProjectAlertGroups(namespace string) ProjectAlertGroupInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectAlertGroupGroupVersionResource, ProjectAlertGroupGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectAlertGroups] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectAlertGroupResource, ProjectAlertGroupGroupVersionKind, projectAlertGroupFactory{})
	return &projectAlertGroupClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterAlertRulesGetter interface {
	ClusterAlertRules(namespace string) ClusterAlertRuleInterface
}

func (c *Client) ClusterAlertRules(namespace string) ClusterAlertRuleInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterAlertRuleGroupVersionResource, ClusterAlertRuleGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterAlertRules] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterAlertRuleResource, ClusterAlertRuleGroupVersionKind, clusterAlertRuleFactory{})
	return &clusterAlertRuleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectAlertRulesGetter interface {
	ProjectAlertRules(namespace string) ProjectAlertRuleInterface
}

func (c *Client) ProjectAlertRules(namespace string) ProjectAlertRuleInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectAlertRuleGroupVersionResource, ProjectAlertRuleGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectAlertRules] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectAlertRuleResource, ProjectAlertRuleGroupVersionKind, projectAlertRuleFactory{})
	return &projectAlertRuleClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ComposeConfigsGetter interface {
	ComposeConfigs(namespace string) ComposeConfigInterface
}

func (c *Client) ComposeConfigs(namespace string) ComposeConfigInterface {
	sharedClient := c.clientFactory.ForResourceKind(ComposeConfigGroupVersionResource, ComposeConfigGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ComposeConfigs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ComposeConfigResource, ComposeConfigGroupVersionKind, composeConfigFactory{})
	return &composeConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectCatalogsGetter interface {
	ProjectCatalogs(namespace string) ProjectCatalogInterface
}

func (c *Client) ProjectCatalogs(namespace string) ProjectCatalogInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectCatalogGroupVersionResource, ProjectCatalogGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectCatalogs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectCatalogResource, ProjectCatalogGroupVersionKind, projectCatalogFactory{})
	return &projectCatalogClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterCatalogsGetter interface {
	ClusterCatalogs(namespace string) ClusterCatalogInterface
}

func (c *Client) ClusterCatalogs(namespace string) ClusterCatalogInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterCatalogGroupVersionResource, ClusterCatalogGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterCatalogs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterCatalogResource, ClusterCatalogGroupVersionKind, clusterCatalogFactory{})
	return &clusterCatalogClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type MultiClusterAppsGetter interface {
	MultiClusterApps(namespace string) MultiClusterAppInterface
}

func (c *Client) MultiClusterApps(namespace string) MultiClusterAppInterface {
	sharedClient := c.clientFactory.ForResourceKind(MultiClusterAppGroupVersionResource, MultiClusterAppGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [MultiClusterApps] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &MultiClusterAppResource, MultiClusterAppGroupVersionKind, multiClusterAppFactory{})
	return &multiClusterAppClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type MultiClusterAppRevisionsGetter interface {
	MultiClusterAppRevisions(namespace string) MultiClusterAppRevisionInterface
}

func (c *Client) MultiClusterAppRevisions(namespace string) MultiClusterAppRevisionInterface {
	sharedClient := c.clientFactory.ForResourceKind(MultiClusterAppRevisionGroupVersionResource, MultiClusterAppRevisionGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [MultiClusterAppRevisions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &MultiClusterAppRevisionResource, MultiClusterAppRevisionGroupVersionKind, multiClusterAppRevisionFactory{})
	return &multiClusterAppRevisionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GlobalDnsesGetter interface {
	GlobalDnses(namespace string) GlobalDnsInterface
}

func (c *Client) GlobalDnses(namespace string) GlobalDnsInterface {
	sharedClient := c.clientFactory.ForResourceKind(GlobalDnsGroupVersionResource, GlobalDnsGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [GlobalDnses] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &GlobalDnsResource, GlobalDnsGroupVersionKind, globalDnsFactory{})
	return &globalDnsClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GlobalDnsProvidersGetter interface {
	GlobalDnsProviders(namespace string) GlobalDnsProviderInterface
}

func (c *Client) GlobalDnsProviders(namespace string) GlobalDnsProviderInterface {
	sharedClient := c.clientFactory.ForResourceKind(GlobalDnsProviderGroupVersionResource, GlobalDnsProviderGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [GlobalDnsProviders] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &GlobalDnsProviderResource, GlobalDnsProviderGroupVersionKind, globalDnsProviderFactory{})
	return &globalDnsProviderClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type KontainerDriversGetter interface {
	KontainerDrivers(namespace string) KontainerDriverInterface
}

func (c *Client) KontainerDrivers(namespace string) KontainerDriverInterface {
	sharedClient := c.clientFactory.ForResourceKind(KontainerDriverGroupVersionResource, KontainerDriverGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [KontainerDrivers] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &KontainerDriverResource, KontainerDriverGroupVersionKind, kontainerDriverFactory{})
	return &kontainerDriverClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type EtcdBackupsGetter interface {
	EtcdBackups(namespace string) EtcdBackupInterface
}

func (c *Client) EtcdBackups(namespace string) EtcdBackupInterface {
	sharedClient := c.clientFactory.ForResourceKind(EtcdBackupGroupVersionResource, EtcdBackupGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [EtcdBackups] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &EtcdBackupResource, EtcdBackupGroupVersionKind, etcdBackupFactory{})
	return &etcdBackupClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterScansGetter interface {
	ClusterScans(namespace string) ClusterScanInterface
}

func (c *Client) ClusterScans(namespace string) ClusterScanInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterScanGroupVersionResource, ClusterScanGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterScans] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterScanResource, ClusterScanGroupVersionKind, clusterScanFactory{})
	return &clusterScanClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type MonitorMetricsGetter interface {
	MonitorMetrics(namespace string) MonitorMetricInterface
}

func (c *Client) MonitorMetrics(namespace string) MonitorMetricInterface {
	sharedClient := c.clientFactory.ForResourceKind(MonitorMetricGroupVersionResource, MonitorMetricGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [MonitorMetrics] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &MonitorMetricResource, MonitorMetricGroupVersionKind, monitorMetricFactory{})
	return &monitorMetricClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterMonitorGraphsGetter interface {
	ClusterMonitorGraphs(namespace string) ClusterMonitorGraphInterface
}

func (c *Client) ClusterMonitorGraphs(namespace string) ClusterMonitorGraphInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterMonitorGraphGroupVersionResource, ClusterMonitorGraphGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterMonitorGraphs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterMonitorGraphResource, ClusterMonitorGraphGroupVersionKind, clusterMonitorGraphFactory{})
	return &clusterMonitorGraphClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectMonitorGraphsGetter interface {
	ProjectMonitorGraphs(namespace string) ProjectMonitorGraphInterface
}

func (c *Client) ProjectMonitorGraphs(namespace string) ProjectMonitorGraphInterface {
	sharedClient := c.clientFactory.ForResourceKind(ProjectMonitorGraphGroupVersionResource, ProjectMonitorGraphGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ProjectMonitorGraphs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ProjectMonitorGraphResource, ProjectMonitorGraphGroupVersionKind, projectMonitorGraphFactory{})
	return &projectMonitorGraphClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CloudCredentialsGetter interface {
	CloudCredentials(namespace string) CloudCredentialInterface
}

func (c *Client) CloudCredentials(namespace string) CloudCredentialInterface {
	sharedClient := c.clientFactory.ForResourceKind(CloudCredentialGroupVersionResource, CloudCredentialGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [CloudCredentials] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CloudCredentialResource, CloudCredentialGroupVersionKind, cloudCredentialFactory{})
	return &cloudCredentialClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterTemplatesGetter interface {
	ClusterTemplates(namespace string) ClusterTemplateInterface
}

func (c *Client) ClusterTemplates(namespace string) ClusterTemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterTemplateGroupVersionResource, ClusterTemplateGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterTemplates] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterTemplateResource, ClusterTemplateGroupVersionKind, clusterTemplateFactory{})
	return &clusterTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterTemplateRevisionsGetter interface {
	ClusterTemplateRevisions(namespace string) ClusterTemplateRevisionInterface
}

func (c *Client) ClusterTemplateRevisions(namespace string) ClusterTemplateRevisionInterface {
	sharedClient := c.clientFactory.ForResourceKind(ClusterTemplateRevisionGroupVersionResource, ClusterTemplateRevisionGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [ClusterTemplateRevisions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &ClusterTemplateRevisionResource, ClusterTemplateRevisionGroupVersionKind, clusterTemplateRevisionFactory{})
	return &clusterTemplateRevisionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RkeK8sSystemImagesGetter interface {
	RkeK8sSystemImages(namespace string) RkeK8sSystemImageInterface
}

func (c *Client) RkeK8sSystemImages(namespace string) RkeK8sSystemImageInterface {
	sharedClient := c.clientFactory.ForResourceKind(RkeK8sSystemImageGroupVersionResource, RkeK8sSystemImageGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [RkeK8sSystemImages] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RkeK8sSystemImageResource, RkeK8sSystemImageGroupVersionKind, rkeK8sSystemImageFactory{})
	return &rkeK8sSystemImageClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RkeK8sServiceOptionsGetter interface {
	RkeK8sServiceOptions(namespace string) RkeK8sServiceOptionInterface
}

func (c *Client) RkeK8sServiceOptions(namespace string) RkeK8sServiceOptionInterface {
	sharedClient := c.clientFactory.ForResourceKind(RkeK8sServiceOptionGroupVersionResource, RkeK8sServiceOptionGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [RkeK8sServiceOptions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RkeK8sServiceOptionResource, RkeK8sServiceOptionGroupVersionKind, rkeK8sServiceOptionFactory{})
	return &rkeK8sServiceOptionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RkeAddonsGetter interface {
	RkeAddons(namespace string) RkeAddonInterface
}

func (c *Client) RkeAddons(namespace string) RkeAddonInterface {
	sharedClient := c.clientFactory.ForResourceKind(RkeAddonGroupVersionResource, RkeAddonGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [RkeAddons] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RkeAddonResource, RkeAddonGroupVersionKind, rkeAddonFactory{})
	return &rkeAddonClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CisConfigsGetter interface {
	CisConfigs(namespace string) CisConfigInterface
}

func (c *Client) CisConfigs(namespace string) CisConfigInterface {
	sharedClient := c.clientFactory.ForResourceKind(CisConfigGroupVersionResource, CisConfigGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [CisConfigs] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CisConfigResource, CisConfigGroupVersionKind, cisConfigFactory{})
	return &cisConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CisBenchmarkVersionsGetter interface {
	CisBenchmarkVersions(namespace string) CisBenchmarkVersionInterface
}

func (c *Client) CisBenchmarkVersions(namespace string) CisBenchmarkVersionInterface {
	sharedClient := c.clientFactory.ForResourceKind(CisBenchmarkVersionGroupVersionResource, CisBenchmarkVersionGroupVersionKind.Kind, true)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [CisBenchmarkVersions] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &CisBenchmarkVersionResource, CisBenchmarkVersionGroupVersionKind, cisBenchmarkVersionFactory{})
	return &cisBenchmarkVersionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type FleetWorkspacesGetter interface {
	FleetWorkspaces(namespace string) FleetWorkspaceInterface
}

func (c *Client) FleetWorkspaces(namespace string) FleetWorkspaceInterface {
	sharedClient := c.clientFactory.ForResourceKind(FleetWorkspaceGroupVersionResource, FleetWorkspaceGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [FleetWorkspaces] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &FleetWorkspaceResource, FleetWorkspaceGroupVersionKind, fleetWorkspaceFactory{})
	return &fleetWorkspaceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RancherUserNotificationsGetter interface {
	RancherUserNotifications(namespace string) RancherUserNotificationInterface
}

func (c *Client) RancherUserNotifications(namespace string) RancherUserNotificationInterface {
	sharedClient := c.clientFactory.ForResourceKind(RancherUserNotificationGroupVersionResource, RancherUserNotificationGroupVersionKind.Kind, false)
	client, err := sharedClient.WithAgent(c.userAgent)
	if err != nil {
		logrus.Errorf("Failed to add user agent to [RancherUserNotifications] client: %v", err)
		client = sharedClient
	}
	objectClient := objectclient.NewObjectClient(namespace, client, &RancherUserNotificationResource, RancherUserNotificationGroupVersionKind, rancherUserNotificationFactory{})
	return &rancherUserNotificationClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
