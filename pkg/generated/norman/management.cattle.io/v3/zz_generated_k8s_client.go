package v3

import (
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Interface interface {
	NodePoolsGetter
	NodesGetter
	NodeDriversGetter
	NodeTemplatesGetter
	PodSecurityAdmissionConfigurationTemplatesGetter
	ProjectsGetter
	GlobalRolesGetter
	GlobalRoleBindingsGetter
	RoleTemplatesGetter
	ClusterRoleTemplateBindingsGetter
	ProjectRoleTemplateBindingsGetter
	ClustersGetter
	ClusterRegistrationTokensGetter
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
	SettingsGetter
	FeaturesGetter
	ComposeConfigsGetter
	KontainerDriversGetter
	EtcdBackupsGetter
	CloudCredentialsGetter
	ClusterTemplatesGetter
	ClusterTemplateRevisionsGetter
	RkeK8sSystemImagesGetter
	RkeK8sServiceOptionsGetter
	RkeAddonsGetter
	FleetWorkspacesGetter
	RancherUserNotificationsGetter
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

type NodePoolsGetter interface {
	NodePools(namespace string) NodePoolInterface
}

func (c *Client) NodePools(namespace string) NodePoolInterface {
	sharedClient := c.clientFactory.ForResourceKind(NodePoolGroupVersionResource, NodePoolGroupVersionKind.Kind, true)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NodePoolResource, NodePoolGroupVersionKind, nodePoolFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NodeResource, NodeGroupVersionKind, nodeFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NodeDriverResource, NodeDriverGroupVersionKind, nodeDriverFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &NodeTemplateResource, NodeTemplateGroupVersionKind, nodeTemplateFactory{})
	return &nodeTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PodSecurityAdmissionConfigurationTemplatesGetter interface {
	PodSecurityAdmissionConfigurationTemplates(namespace string) PodSecurityAdmissionConfigurationTemplateInterface
}

func (c *Client) PodSecurityAdmissionConfigurationTemplates(namespace string) PodSecurityAdmissionConfigurationTemplateInterface {
	sharedClient := c.clientFactory.ForResourceKind(PodSecurityAdmissionConfigurationTemplateGroupVersionResource, PodSecurityAdmissionConfigurationTemplateGroupVersionKind.Kind, false)
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PodSecurityAdmissionConfigurationTemplateResource, PodSecurityAdmissionConfigurationTemplateGroupVersionKind, podSecurityAdmissionConfigurationTemplateFactory{})
	return &podSecurityAdmissionConfigurationTemplateClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ProjectResource, ProjectGroupVersionKind, projectFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &GlobalRoleResource, GlobalRoleGroupVersionKind, globalRoleFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &GlobalRoleBindingResource, GlobalRoleBindingGroupVersionKind, globalRoleBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &RoleTemplateResource, RoleTemplateGroupVersionKind, roleTemplateFactory{})
	return &roleTemplateClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterRoleTemplateBindingResource, ClusterRoleTemplateBindingGroupVersionKind, clusterRoleTemplateBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ProjectRoleTemplateBindingResource, ProjectRoleTemplateBindingGroupVersionKind, projectRoleTemplateBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterResource, ClusterGroupVersionKind, clusterFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterRegistrationTokenResource, ClusterRegistrationTokenGroupVersionKind, clusterRegistrationTokenFactory{})
	return &clusterRegistrationTokenClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &GroupResource, GroupGroupVersionKind, groupFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &GroupMemberResource, GroupMemberGroupVersionKind, groupMemberFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SamlTokenResource, SamlTokenGroupVersionKind, samlTokenFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PrincipalResource, PrincipalGroupVersionKind, principalFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &UserResource, UserGroupVersionKind, userFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &AuthConfigResource, AuthConfigGroupVersionKind, authConfigFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &LdapConfigResource, LdapConfigGroupVersionKind, ldapConfigFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &TokenResource, TokenGroupVersionKind, tokenFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &DynamicSchemaResource, DynamicSchemaGroupVersionKind, dynamicSchemaFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &PreferenceResource, PreferenceGroupVersionKind, preferenceFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &UserAttributeResource, UserAttributeGroupVersionKind, userAttributeFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ProjectNetworkPolicyResource, ProjectNetworkPolicyGroupVersionKind, projectNetworkPolicyFactory{})
	return &projectNetworkPolicyClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &SettingResource, SettingGroupVersionKind, settingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &FeatureResource, FeatureGroupVersionKind, featureFactory{})
	return &featureClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ComposeConfigResource, ComposeConfigGroupVersionKind, composeConfigFactory{})
	return &composeConfigClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &KontainerDriverResource, KontainerDriverGroupVersionKind, kontainerDriverFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &EtcdBackupResource, EtcdBackupGroupVersionKind, etcdBackupFactory{})
	return &etcdBackupClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &CloudCredentialResource, CloudCredentialGroupVersionKind, cloudCredentialFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterTemplateResource, ClusterTemplateGroupVersionKind, clusterTemplateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &ClusterTemplateRevisionResource, ClusterTemplateRevisionGroupVersionKind, clusterTemplateRevisionFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &RkeK8sSystemImageResource, RkeK8sSystemImageGroupVersionKind, rkeK8sSystemImageFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &RkeK8sServiceOptionResource, RkeK8sServiceOptionGroupVersionKind, rkeK8sServiceOptionFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &RkeAddonResource, RkeAddonGroupVersionKind, rkeAddonFactory{})
	return &rkeAddonClient{
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &FleetWorkspaceResource, FleetWorkspaceGroupVersionKind, fleetWorkspaceFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, sharedClient, &RancherUserNotificationResource, RancherUserNotificationGroupVersionKind, rancherUserNotificationFactory{})
	return &rancherUserNotificationClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
