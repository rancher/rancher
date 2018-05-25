package v3

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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
	ClusterEventsGetter
	ClusterRegistrationTokensGetter
	CatalogsGetter
	TemplatesGetter
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
	ListenConfigsGetter
	SettingsGetter
	NotifiersGetter
	ClusterAlertsGetter
	ProjectAlertsGetter
	ClusterPipelinesGetter
	SourceCodeCredentialsGetter
	PipelinesGetter
	PipelineExecutionsGetter
	PipelineExecutionLogsGetter
	SourceCodeRepositoriesGetter
	ComposeConfigsGetter
	ResourceQuotaTemplatesGetter
	CattleInstancesGetter
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
	clusterEventControllers                            map[string]ClusterEventController
	clusterRegistrationTokenControllers                map[string]ClusterRegistrationTokenController
	catalogControllers                                 map[string]CatalogController
	templateControllers                                map[string]TemplateController
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
	listenConfigControllers                            map[string]ListenConfigController
	settingControllers                                 map[string]SettingController
	notifierControllers                                map[string]NotifierController
	clusterAlertControllers                            map[string]ClusterAlertController
	projectAlertControllers                            map[string]ProjectAlertController
	clusterPipelineControllers                         map[string]ClusterPipelineController
	sourceCodeCredentialControllers                    map[string]SourceCodeCredentialController
	pipelineControllers                                map[string]PipelineController
	pipelineExecutionControllers                       map[string]PipelineExecutionController
	pipelineExecutionLogControllers                    map[string]PipelineExecutionLogController
	sourceCodeRepositoryControllers                    map[string]SourceCodeRepositoryController
	composeConfigControllers                           map[string]ComposeConfigController
	resourceQuotaTemplateControllers                   map[string]ResourceQuotaTemplateController
	cattleInstanceControllers                          map[string]CattleInstanceController
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		config.NegotiatedSerializer = configConfig.NegotiatedSerializer
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
		clusterEventControllers:                            map[string]ClusterEventController{},
		clusterRegistrationTokenControllers:                map[string]ClusterRegistrationTokenController{},
		catalogControllers:                                 map[string]CatalogController{},
		templateControllers:                                map[string]TemplateController{},
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
		listenConfigControllers:                            map[string]ListenConfigController{},
		settingControllers:                                 map[string]SettingController{},
		notifierControllers:                                map[string]NotifierController{},
		clusterAlertControllers:                            map[string]ClusterAlertController{},
		projectAlertControllers:                            map[string]ProjectAlertController{},
		clusterPipelineControllers:                         map[string]ClusterPipelineController{},
		sourceCodeCredentialControllers:                    map[string]SourceCodeCredentialController{},
		pipelineControllers:                                map[string]PipelineController{},
		pipelineExecutionControllers:                       map[string]PipelineExecutionController{},
		pipelineExecutionLogControllers:                    map[string]PipelineExecutionLogController{},
		sourceCodeRepositoryControllers:                    map[string]SourceCodeRepositoryController{},
		composeConfigControllers:                           map[string]ComposeConfigController{},
		resourceQuotaTemplateControllers:                   map[string]ResourceQuotaTemplateController{},
		cattleInstanceControllers:                          map[string]CattleInstanceController{},
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

type ClusterEventsGetter interface {
	ClusterEvents(namespace string) ClusterEventInterface
}

func (c *Client) ClusterEvents(namespace string) ClusterEventInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterEventResource, ClusterEventGroupVersionKind, clusterEventFactory{})
	return &clusterEventClient{
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

type ListenConfigsGetter interface {
	ListenConfigs(namespace string) ListenConfigInterface
}

func (c *Client) ListenConfigs(namespace string) ListenConfigInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ListenConfigResource, ListenConfigGroupVersionKind, listenConfigFactory{})
	return &listenConfigClient{
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

type ClusterPipelinesGetter interface {
	ClusterPipelines(namespace string) ClusterPipelineInterface
}

func (c *Client) ClusterPipelines(namespace string) ClusterPipelineInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ClusterPipelineResource, ClusterPipelineGroupVersionKind, clusterPipelineFactory{})
	return &clusterPipelineClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SourceCodeCredentialsGetter interface {
	SourceCodeCredentials(namespace string) SourceCodeCredentialInterface
}

func (c *Client) SourceCodeCredentials(namespace string) SourceCodeCredentialInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &SourceCodeCredentialResource, SourceCodeCredentialGroupVersionKind, sourceCodeCredentialFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PipelineResource, PipelineGroupVersionKind, pipelineFactory{})
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
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PipelineExecutionResource, PipelineExecutionGroupVersionKind, pipelineExecutionFactory{})
	return &pipelineExecutionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PipelineExecutionLogsGetter interface {
	PipelineExecutionLogs(namespace string) PipelineExecutionLogInterface
}

func (c *Client) PipelineExecutionLogs(namespace string) PipelineExecutionLogInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PipelineExecutionLogResource, PipelineExecutionLogGroupVersionKind, pipelineExecutionLogFactory{})
	return &pipelineExecutionLogClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type SourceCodeRepositoriesGetter interface {
	SourceCodeRepositories(namespace string) SourceCodeRepositoryInterface
}

func (c *Client) SourceCodeRepositories(namespace string) SourceCodeRepositoryInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &SourceCodeRepositoryResource, SourceCodeRepositoryGroupVersionKind, sourceCodeRepositoryFactory{})
	return &sourceCodeRepositoryClient{
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

type ResourceQuotaTemplatesGetter interface {
	ResourceQuotaTemplates(namespace string) ResourceQuotaTemplateInterface
}

func (c *Client) ResourceQuotaTemplates(namespace string) ResourceQuotaTemplateInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ResourceQuotaTemplateResource, ResourceQuotaTemplateGroupVersionKind, resourceQuotaTemplateFactory{})
	return &resourceQuotaTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type CattleInstancesGetter interface {
	CattleInstances(namespace string) CattleInstanceInterface
}

func (c *Client) CattleInstances(namespace string) CattleInstanceInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &CattleInstanceResource, CattleInstanceGroupVersionKind, cattleInstanceFactory{})
	return &cattleInstanceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
