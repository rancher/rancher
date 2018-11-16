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
	ComposeConfigsGetter
	ProjectCatalogsGetter
	ClusterCatalogsGetter
	KontainerDriversGetter
}

type Clients struct {
	NodePool                                NodePoolClient
	Node                                    NodeClient
	NodeDriver                              NodeDriverClient
	NodeTemplate                            NodeTemplateClient
	Project                                 ProjectClient
	GlobalRole                              GlobalRoleClient
	GlobalRoleBinding                       GlobalRoleBindingClient
	RoleTemplate                            RoleTemplateClient
	PodSecurityPolicyTemplate               PodSecurityPolicyTemplateClient
	PodSecurityPolicyTemplateProjectBinding PodSecurityPolicyTemplateProjectBindingClient
	ClusterRoleTemplateBinding              ClusterRoleTemplateBindingClient
	ProjectRoleTemplateBinding              ProjectRoleTemplateBindingClient
	Cluster                                 ClusterClient
	ClusterRegistrationToken                ClusterRegistrationTokenClient
	Catalog                                 CatalogClient
	Template                                TemplateClient
	TemplateVersion                         TemplateVersionClient
	TemplateContent                         TemplateContentClient
	Group                                   GroupClient
	GroupMember                             GroupMemberClient
	Principal                               PrincipalClient
	User                                    UserClient
	AuthConfig                              AuthConfigClient
	LdapConfig                              LdapConfigClient
	Token                                   TokenClient
	DynamicSchema                           DynamicSchemaClient
	Preference                              PreferenceClient
	UserAttribute                           UserAttributeClient
	ProjectNetworkPolicy                    ProjectNetworkPolicyClient
	ClusterLogging                          ClusterLoggingClient
	ProjectLogging                          ProjectLoggingClient
	ListenConfig                            ListenConfigClient
	Setting                                 SettingClient
	Notifier                                NotifierClient
	ClusterAlert                            ClusterAlertClient
	ProjectAlert                            ProjectAlertClient
	ComposeConfig                           ComposeConfigClient
	ProjectCatalog                          ProjectCatalogClient
	ClusterCatalog                          ClusterCatalogClient
	KontainerDriver                         KontainerDriverClient
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
	composeConfigControllers                           map[string]ComposeConfigController
	projectCatalogControllers                          map[string]ProjectCatalogController
	clusterCatalogControllers                          map[string]ClusterCatalogController
	kontainerDriverControllers                         map[string]KontainerDriverController
}

func Factory(ctx context.Context, config rest.Config) (context.Context, controller.Starter, error) {
	c, err := NewForConfig(config)
	if err != nil {
		return ctx, nil, err
	}

	cs := NewClientsFromInterface(c)

	ctx = context.WithValue(ctx, contextKeyType{}, c)
	ctx = context.WithValue(ctx, contextClientsKeyType{}, cs)
	return ctx, c, nil
}

func ClientsFrom(ctx context.Context) *Clients {
	return ctx.Value(contextClientsKeyType{}).(*Clients)
}

func From(ctx context.Context) Interface {
	return ctx.Value(contextKeyType{}).(Interface)
}

func NewClients(config rest.Config) (*Clients, error) {
	iface, err := NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewClientsFromInterface(iface), nil
}

func NewClientsFromInterface(iface Interface) *Clients {
	return &Clients{

		NodePool: &nodePoolClient2{
			iface: iface.NodePools(""),
		},
		Node: &nodeClient2{
			iface: iface.Nodes(""),
		},
		NodeDriver: &nodeDriverClient2{
			iface: iface.NodeDrivers(""),
		},
		NodeTemplate: &nodeTemplateClient2{
			iface: iface.NodeTemplates(""),
		},
		Project: &projectClient2{
			iface: iface.Projects(""),
		},
		GlobalRole: &globalRoleClient2{
			iface: iface.GlobalRoles(""),
		},
		GlobalRoleBinding: &globalRoleBindingClient2{
			iface: iface.GlobalRoleBindings(""),
		},
		RoleTemplate: &roleTemplateClient2{
			iface: iface.RoleTemplates(""),
		},
		PodSecurityPolicyTemplate: &podSecurityPolicyTemplateClient2{
			iface: iface.PodSecurityPolicyTemplates(""),
		},
		PodSecurityPolicyTemplateProjectBinding: &podSecurityPolicyTemplateProjectBindingClient2{
			iface: iface.PodSecurityPolicyTemplateProjectBindings(""),
		},
		ClusterRoleTemplateBinding: &clusterRoleTemplateBindingClient2{
			iface: iface.ClusterRoleTemplateBindings(""),
		},
		ProjectRoleTemplateBinding: &projectRoleTemplateBindingClient2{
			iface: iface.ProjectRoleTemplateBindings(""),
		},
		Cluster: &clusterClient2{
			iface: iface.Clusters(""),
		},
		ClusterRegistrationToken: &clusterRegistrationTokenClient2{
			iface: iface.ClusterRegistrationTokens(""),
		},
		Catalog: &catalogClient2{
			iface: iface.Catalogs(""),
		},
		Template: &templateClient2{
			iface: iface.Templates(""),
		},
		TemplateVersion: &templateVersionClient2{
			iface: iface.TemplateVersions(""),
		},
		TemplateContent: &templateContentClient2{
			iface: iface.TemplateContents(""),
		},
		Group: &groupClient2{
			iface: iface.Groups(""),
		},
		GroupMember: &groupMemberClient2{
			iface: iface.GroupMembers(""),
		},
		Principal: &principalClient2{
			iface: iface.Principals(""),
		},
		User: &userClient2{
			iface: iface.Users(""),
		},
		AuthConfig: &authConfigClient2{
			iface: iface.AuthConfigs(""),
		},
		LdapConfig: &ldapConfigClient2{
			iface: iface.LdapConfigs(""),
		},
		Token: &tokenClient2{
			iface: iface.Tokens(""),
		},
		DynamicSchema: &dynamicSchemaClient2{
			iface: iface.DynamicSchemas(""),
		},
		Preference: &preferenceClient2{
			iface: iface.Preferences(""),
		},
		UserAttribute: &userAttributeClient2{
			iface: iface.UserAttributes(""),
		},
		ProjectNetworkPolicy: &projectNetworkPolicyClient2{
			iface: iface.ProjectNetworkPolicies(""),
		},
		ClusterLogging: &clusterLoggingClient2{
			iface: iface.ClusterLoggings(""),
		},
		ProjectLogging: &projectLoggingClient2{
			iface: iface.ProjectLoggings(""),
		},
		ListenConfig: &listenConfigClient2{
			iface: iface.ListenConfigs(""),
		},
		Setting: &settingClient2{
			iface: iface.Settings(""),
		},
		Notifier: &notifierClient2{
			iface: iface.Notifiers(""),
		},
		ClusterAlert: &clusterAlertClient2{
			iface: iface.ClusterAlerts(""),
		},
		ProjectAlert: &projectAlertClient2{
			iface: iface.ProjectAlerts(""),
		},
		ComposeConfig: &composeConfigClient2{
			iface: iface.ComposeConfigs(""),
		},
		ProjectCatalog: &projectCatalogClient2{
			iface: iface.ProjectCatalogs(""),
		},
		ClusterCatalog: &clusterCatalogClient2{
			iface: iface.ClusterCatalogs(""),
		},
		KontainerDriver: &kontainerDriverClient2{
			iface: iface.KontainerDrivers(""),
		},
	}
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
		composeConfigControllers:                           map[string]ComposeConfigController{},
		projectCatalogControllers:                          map[string]ProjectCatalogController{},
		clusterCatalogControllers:                          map[string]ClusterCatalogController{},
		kontainerDriverControllers:                         map[string]KontainerDriverController{},
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
