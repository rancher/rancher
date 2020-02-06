package v3

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/rest"
)

type (
	contextKeyType        struct{}
	contextClientsKeyType struct{}
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

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
	GlobalDNSsGetter
	GlobalDNSProvidersGetter
	KontainerDriversGetter
	EtcdBackupsGetter
	ClusterScansGetter
	MonitorMetricsGetter
	ClusterMonitorGraphsGetter
	ProjectMonitorGraphsGetter
	CloudCredentialsGetter
	ClusterTemplatesGetter
	ClusterTemplateRevisionsGetter
	RKEK8sSystemImagesGetter
	RKEK8sServiceOptionsGetter
	RKEAddonsGetter
	CisConfigsGetter
	CisBenchmarkVersionsGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	nodePoolControllers                                map[string]NodePoolController
	nodeControllers                                    map[string]NodeController
	nodeDriverControllers                              map[string]NodeDriverController
	nodeTemplateControllers                            map[string]NodeTemplateController
	projectControllers                                 map[string]ProjectController
	globalRoleControllers                              map[string]GlobalRoleController
	globalRoleBindingControllers                       map[string]GlobalRoleBindingController
	roleTemplateControllers                            map[string]RoleTemplateController
	podSecurityPolicyTemplateControllers               map[string]PodSecurityPolicyTemplateController
	podSecurityPolicyTemplateProjectBindingControllers map[string]PodSecurityPolicyTemplateProjectBindingController
	clusterRoleTemplateBindingControllers              map[string]ClusterRoleTemplateBindingController
	projectRoleTemplateBindingControllers              map[string]ProjectRoleTemplateBindingController
	clusterControllers                                 map[string]ClusterController
	clusterRegistrationTokenControllers                map[string]ClusterRegistrationTokenController
	catalogControllers                                 map[string]CatalogController
	templateControllers                                map[string]TemplateController
	catalogTemplateControllers                         map[string]CatalogTemplateController
	catalogTemplateVersionControllers                  map[string]CatalogTemplateVersionController
	templateVersionControllers                         map[string]TemplateVersionController
	templateContentControllers                         map[string]TemplateContentController
	groupControllers                                   map[string]GroupController
	groupMemberControllers                             map[string]GroupMemberController
	principalControllers                               map[string]PrincipalController
	userControllers                                    map[string]UserController
	authConfigControllers                              map[string]AuthConfigController
	ldapConfigControllers                              map[string]LdapConfigController
	tokenControllers                                   map[string]TokenController
	dynamicSchemaControllers                           map[string]DynamicSchemaController
	preferenceControllers                              map[string]PreferenceController
	userAttributeControllers                           map[string]UserAttributeController
	projectNetworkPolicyControllers                    map[string]ProjectNetworkPolicyController
	clusterLoggingControllers                          map[string]ClusterLoggingController
	projectLoggingControllers                          map[string]ProjectLoggingController
	settingControllers                                 map[string]SettingController
	featureControllers                                 map[string]FeatureController
	clusterAlertControllers                            map[string]ClusterAlertController
	projectAlertControllers                            map[string]ProjectAlertController
	notifierControllers                                map[string]NotifierController
	clusterAlertGroupControllers                       map[string]ClusterAlertGroupController
	projectAlertGroupControllers                       map[string]ProjectAlertGroupController
	clusterAlertRuleControllers                        map[string]ClusterAlertRuleController
	projectAlertRuleControllers                        map[string]ProjectAlertRuleController
	composeConfigControllers                           map[string]ComposeConfigController
	projectCatalogControllers                          map[string]ProjectCatalogController
	clusterCatalogControllers                          map[string]ClusterCatalogController
	multiClusterAppControllers                         map[string]MultiClusterAppController
	multiClusterAppRevisionControllers                 map[string]MultiClusterAppRevisionController
	globalDnsControllers                               map[string]GlobalDNSController
	globalDnsProviderControllers                       map[string]GlobalDNSProviderController
	kontainerDriverControllers                         map[string]KontainerDriverController
	etcdBackupControllers                              map[string]EtcdBackupController
	clusterScanControllers                             map[string]ClusterScanController
	monitorMetricControllers                           map[string]MonitorMetricController
	clusterMonitorGraphControllers                     map[string]ClusterMonitorGraphController
	projectMonitorGraphControllers                     map[string]ProjectMonitorGraphController
	cloudCredentialControllers                         map[string]CloudCredentialController
	clusterTemplateControllers                         map[string]ClusterTemplateController
	clusterTemplateRevisionControllers                 map[string]ClusterTemplateRevisionController
	rkeK8sSystemImageControllers                       map[string]RKEK8sSystemImageController
	rkeK8sServiceOptionControllers                     map[string]RKEK8sServiceOptionController
	rkeAddonControllers                                map[string]RKEAddonController
	cisConfigControllers                               map[string]CisConfigController
	cisBenchmarkVersionControllers                     map[string]CisBenchmarkVersionController
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

		nodePoolControllers:                                map[string]NodePoolController{},
		nodeControllers:                                    map[string]NodeController{},
		nodeDriverControllers:                              map[string]NodeDriverController{},
		nodeTemplateControllers:                            map[string]NodeTemplateController{},
		projectControllers:                                 map[string]ProjectController{},
		globalRoleControllers:                              map[string]GlobalRoleController{},
		globalRoleBindingControllers:                       map[string]GlobalRoleBindingController{},
		roleTemplateControllers:                            map[string]RoleTemplateController{},
		podSecurityPolicyTemplateControllers:               map[string]PodSecurityPolicyTemplateController{},
		podSecurityPolicyTemplateProjectBindingControllers: map[string]PodSecurityPolicyTemplateProjectBindingController{},
		clusterRoleTemplateBindingControllers:              map[string]ClusterRoleTemplateBindingController{},
		projectRoleTemplateBindingControllers:              map[string]ProjectRoleTemplateBindingController{},
		clusterControllers:                                 map[string]ClusterController{},
		clusterRegistrationTokenControllers:                map[string]ClusterRegistrationTokenController{},
		catalogControllers:                                 map[string]CatalogController{},
		templateControllers:                                map[string]TemplateController{},
		catalogTemplateControllers:                         map[string]CatalogTemplateController{},
		catalogTemplateVersionControllers:                  map[string]CatalogTemplateVersionController{},
		templateVersionControllers:                         map[string]TemplateVersionController{},
		templateContentControllers:                         map[string]TemplateContentController{},
		groupControllers:                                   map[string]GroupController{},
		groupMemberControllers:                             map[string]GroupMemberController{},
		principalControllers:                               map[string]PrincipalController{},
		userControllers:                                    map[string]UserController{},
		authConfigControllers:                              map[string]AuthConfigController{},
		ldapConfigControllers:                              map[string]LdapConfigController{},
		tokenControllers:                                   map[string]TokenController{},
		dynamicSchemaControllers:                           map[string]DynamicSchemaController{},
		preferenceControllers:                              map[string]PreferenceController{},
		userAttributeControllers:                           map[string]UserAttributeController{},
		projectNetworkPolicyControllers:                    map[string]ProjectNetworkPolicyController{},
		clusterLoggingControllers:                          map[string]ClusterLoggingController{},
		projectLoggingControllers:                          map[string]ProjectLoggingController{},
		settingControllers:                                 map[string]SettingController{},
		featureControllers:                                 map[string]FeatureController{},
		clusterAlertControllers:                            map[string]ClusterAlertController{},
		projectAlertControllers:                            map[string]ProjectAlertController{},
		notifierControllers:                                map[string]NotifierController{},
		clusterAlertGroupControllers:                       map[string]ClusterAlertGroupController{},
		projectAlertGroupControllers:                       map[string]ProjectAlertGroupController{},
		clusterAlertRuleControllers:                        map[string]ClusterAlertRuleController{},
		projectAlertRuleControllers:                        map[string]ProjectAlertRuleController{},
		composeConfigControllers:                           map[string]ComposeConfigController{},
		projectCatalogControllers:                          map[string]ProjectCatalogController{},
		clusterCatalogControllers:                          map[string]ClusterCatalogController{},
		multiClusterAppControllers:                         map[string]MultiClusterAppController{},
		multiClusterAppRevisionControllers:                 map[string]MultiClusterAppRevisionController{},
		globalDnsControllers:                               map[string]GlobalDNSController{},
		globalDnsProviderControllers:                       map[string]GlobalDNSProviderController{},
		kontainerDriverControllers:                         map[string]KontainerDriverController{},
		etcdBackupControllers:                              map[string]EtcdBackupController{},
		clusterScanControllers:                             map[string]ClusterScanController{},
		monitorMetricControllers:                           map[string]MonitorMetricController{},
		clusterMonitorGraphControllers:                     map[string]ClusterMonitorGraphController{},
		projectMonitorGraphControllers:                     map[string]ProjectMonitorGraphController{},
		cloudCredentialControllers:                         map[string]CloudCredentialController{},
		clusterTemplateControllers:                         map[string]ClusterTemplateController{},
		clusterTemplateRevisionControllers:                 map[string]ClusterTemplateRevisionController{},
		rkeK8sSystemImageControllers:                       map[string]RKEK8sSystemImageController{},
		rkeK8sServiceOptionControllers:                     map[string]RKEK8sServiceOptionController{},
		rkeAddonControllers:                                map[string]RKEAddonController{},
		cisConfigControllers:                               map[string]CisConfigController{},
		cisBenchmarkVersionControllers:                     map[string]CisBenchmarkVersionController{},
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

type NodePoolsGetter interface {
	NodePools(namespace string) NodePoolInterface
}

func (c *Client) NodePools(namespace string) NodePoolInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NodePoolResource, NodePoolGroupVersionKind, nodePoolFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NodeResource, NodeGroupVersionKind, nodeFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NodeDriverResource, NodeDriverGroupVersionKind, nodeDriverFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NodeTemplateResource, NodeTemplateGroupVersionKind, nodeTemplateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectResource, ProjectGroupVersionKind, projectFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &GlobalRoleResource, GlobalRoleGroupVersionKind, globalRoleFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &GlobalRoleBindingResource, GlobalRoleBindingGroupVersionKind, globalRoleBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &RoleTemplateResource, RoleTemplateGroupVersionKind, roleTemplateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PodSecurityPolicyTemplateResource, PodSecurityPolicyTemplateGroupVersionKind, podSecurityPolicyTemplateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PodSecurityPolicyTemplateProjectBindingResource, PodSecurityPolicyTemplateProjectBindingGroupVersionKind, podSecurityPolicyTemplateProjectBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterRoleTemplateBindingResource, ClusterRoleTemplateBindingGroupVersionKind, clusterRoleTemplateBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectRoleTemplateBindingResource, ProjectRoleTemplateBindingGroupVersionKind, projectRoleTemplateBindingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterResource, ClusterGroupVersionKind, clusterFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterRegistrationTokenResource, ClusterRegistrationTokenGroupVersionKind, clusterRegistrationTokenFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CatalogResource, CatalogGroupVersionKind, catalogFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &TemplateResource, TemplateGroupVersionKind, templateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CatalogTemplateResource, CatalogTemplateGroupVersionKind, catalogTemplateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CatalogTemplateVersionResource, CatalogTemplateVersionGroupVersionKind, catalogTemplateVersionFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &TemplateVersionResource, TemplateVersionGroupVersionKind, templateVersionFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &TemplateContentResource, TemplateContentGroupVersionKind, templateContentFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &GroupResource, GroupGroupVersionKind, groupFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &GroupMemberResource, GroupMemberGroupVersionKind, groupMemberFactory{})
	return &groupMemberClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PrincipalsGetter interface {
	Principals(namespace string) PrincipalInterface
}

func (c *Client) Principals(namespace string) PrincipalInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PrincipalResource, PrincipalGroupVersionKind, principalFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &UserResource, UserGroupVersionKind, userFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &AuthConfigResource, AuthConfigGroupVersionKind, authConfigFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &LdapConfigResource, LdapConfigGroupVersionKind, ldapConfigFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &TokenResource, TokenGroupVersionKind, tokenFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &DynamicSchemaResource, DynamicSchemaGroupVersionKind, dynamicSchemaFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PreferenceResource, PreferenceGroupVersionKind, preferenceFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &UserAttributeResource, UserAttributeGroupVersionKind, userAttributeFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectNetworkPolicyResource, ProjectNetworkPolicyGroupVersionKind, projectNetworkPolicyFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterLoggingResource, ClusterLoggingGroupVersionKind, clusterLoggingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectLoggingResource, ProjectLoggingGroupVersionKind, projectLoggingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &SettingResource, SettingGroupVersionKind, settingFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &FeatureResource, FeatureGroupVersionKind, featureFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterAlertResource, ClusterAlertGroupVersionKind, clusterAlertFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectAlertResource, ProjectAlertGroupVersionKind, projectAlertFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NotifierResource, NotifierGroupVersionKind, notifierFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterAlertGroupResource, ClusterAlertGroupGroupVersionKind, clusterAlertGroupFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectAlertGroupResource, ProjectAlertGroupGroupVersionKind, projectAlertGroupFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterAlertRuleResource, ClusterAlertRuleGroupVersionKind, clusterAlertRuleFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectAlertRuleResource, ProjectAlertRuleGroupVersionKind, projectAlertRuleFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ComposeConfigResource, ComposeConfigGroupVersionKind, composeConfigFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectCatalogResource, ProjectCatalogGroupVersionKind, projectCatalogFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterCatalogResource, ClusterCatalogGroupVersionKind, clusterCatalogFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &MultiClusterAppResource, MultiClusterAppGroupVersionKind, multiClusterAppFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &MultiClusterAppRevisionResource, MultiClusterAppRevisionGroupVersionKind, multiClusterAppRevisionFactory{})
	return &multiClusterAppRevisionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GlobalDNSsGetter interface {
	GlobalDNSs(namespace string) GlobalDNSInterface
}

func (c *Client) GlobalDNSs(namespace string) GlobalDNSInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &GlobalDNSResource, GlobalDNSGroupVersionKind, globalDnsFactory{})
	return &globalDnsClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GlobalDNSProvidersGetter interface {
	GlobalDNSProviders(namespace string) GlobalDNSProviderInterface
}

func (c *Client) GlobalDNSProviders(namespace string) GlobalDNSProviderInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &GlobalDNSProviderResource, GlobalDNSProviderGroupVersionKind, globalDnsProviderFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &KontainerDriverResource, KontainerDriverGroupVersionKind, kontainerDriverFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &EtcdBackupResource, EtcdBackupGroupVersionKind, etcdBackupFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterScanResource, ClusterScanGroupVersionKind, clusterScanFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &MonitorMetricResource, MonitorMetricGroupVersionKind, monitorMetricFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterMonitorGraphResource, ClusterMonitorGraphGroupVersionKind, clusterMonitorGraphFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ProjectMonitorGraphResource, ProjectMonitorGraphGroupVersionKind, projectMonitorGraphFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CloudCredentialResource, CloudCredentialGroupVersionKind, cloudCredentialFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterTemplateResource, ClusterTemplateGroupVersionKind, clusterTemplateFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterTemplateRevisionResource, ClusterTemplateRevisionGroupVersionKind, clusterTemplateRevisionFactory{})
	return &clusterTemplateRevisionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RKEK8sSystemImagesGetter interface {
	RKEK8sSystemImages(namespace string) RKEK8sSystemImageInterface
}

func (c *Client) RKEK8sSystemImages(namespace string) RKEK8sSystemImageInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &RKEK8sSystemImageResource, RKEK8sSystemImageGroupVersionKind, rkeK8sSystemImageFactory{})
	return &rkeK8sSystemImageClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RKEK8sServiceOptionsGetter interface {
	RKEK8sServiceOptions(namespace string) RKEK8sServiceOptionInterface
}

func (c *Client) RKEK8sServiceOptions(namespace string) RKEK8sServiceOptionInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &RKEK8sServiceOptionResource, RKEK8sServiceOptionGroupVersionKind, rkeK8sServiceOptionFactory{})
	return &rkeK8sServiceOptionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type RKEAddonsGetter interface {
	RKEAddons(namespace string) RKEAddonInterface
}

func (c *Client) RKEAddons(namespace string) RKEAddonInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &RKEAddonResource, RKEAddonGroupVersionKind, rkeAddonFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CisConfigResource, CisConfigGroupVersionKind, cisConfigFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CisBenchmarkVersionResource, CisBenchmarkVersionGroupVersionKind, cisBenchmarkVersionFactory{})
	return &cisBenchmarkVersionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
