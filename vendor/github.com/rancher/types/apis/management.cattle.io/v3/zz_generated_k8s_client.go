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

	MachinesGetter
	MachineDriversGetter
	MachineTemplatesGetter
	ProjectsGetter
	GlobalRolesGetter
	GlobalRoleBindingsGetter
	RoleTemplatesGetter
	PodSecurityPolicyTemplatesGetter
	ClusterRoleTemplateBindingsGetter
	ProjectRoleTemplateBindingsGetter
	ClustersGetter
	ClusterEventsGetter
	ClusterRegistrationTokensGetter
	CatalogsGetter
	TemplatesGetter
	TemplateVersionsGetter
	GroupsGetter
	GroupMembersGetter
	PrincipalsGetter
	UsersGetter
	AuthConfigsGetter
	TokensGetter
	DynamicSchemasGetter
	PreferencesGetter
	ClusterLoggingsGetter
	ProjectLoggingsGetter
	ListenConfigsGetter
	SettingsGetter
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	machineControllers                    map[string]MachineController
	machineDriverControllers              map[string]MachineDriverController
	machineTemplateControllers            map[string]MachineTemplateController
	projectControllers                    map[string]ProjectController
	globalRoleControllers                 map[string]GlobalRoleController
	globalRoleBindingControllers          map[string]GlobalRoleBindingController
	roleTemplateControllers               map[string]RoleTemplateController
	podSecurityPolicyTemplateControllers  map[string]PodSecurityPolicyTemplateController
	clusterRoleTemplateBindingControllers map[string]ClusterRoleTemplateBindingController
	projectRoleTemplateBindingControllers map[string]ProjectRoleTemplateBindingController
	clusterControllers                    map[string]ClusterController
	clusterEventControllers               map[string]ClusterEventController
	clusterRegistrationTokenControllers   map[string]ClusterRegistrationTokenController
	catalogControllers                    map[string]CatalogController
	templateControllers                   map[string]TemplateController
	templateVersionControllers            map[string]TemplateVersionController
	groupControllers                      map[string]GroupController
	groupMemberControllers                map[string]GroupMemberController
	principalControllers                  map[string]PrincipalController
	userControllers                       map[string]UserController
	authConfigControllers                 map[string]AuthConfigController
	tokenControllers                      map[string]TokenController
	dynamicSchemaControllers              map[string]DynamicSchemaController
	preferenceControllers                 map[string]PreferenceController
	clusterLoggingControllers             map[string]ClusterLoggingController
	projectLoggingControllers             map[string]ProjectLoggingController
	listenConfigControllers               map[string]ListenConfigController
	settingControllers                    map[string]SettingController
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

		machineControllers:                    map[string]MachineController{},
		machineDriverControllers:              map[string]MachineDriverController{},
		machineTemplateControllers:            map[string]MachineTemplateController{},
		projectControllers:                    map[string]ProjectController{},
		globalRoleControllers:                 map[string]GlobalRoleController{},
		globalRoleBindingControllers:          map[string]GlobalRoleBindingController{},
		roleTemplateControllers:               map[string]RoleTemplateController{},
		podSecurityPolicyTemplateControllers:  map[string]PodSecurityPolicyTemplateController{},
		clusterRoleTemplateBindingControllers: map[string]ClusterRoleTemplateBindingController{},
		projectRoleTemplateBindingControllers: map[string]ProjectRoleTemplateBindingController{},
		clusterControllers:                    map[string]ClusterController{},
		clusterEventControllers:               map[string]ClusterEventController{},
		clusterRegistrationTokenControllers:   map[string]ClusterRegistrationTokenController{},
		catalogControllers:                    map[string]CatalogController{},
		templateControllers:                   map[string]TemplateController{},
		templateVersionControllers:            map[string]TemplateVersionController{},
		groupControllers:                      map[string]GroupController{},
		groupMemberControllers:                map[string]GroupMemberController{},
		principalControllers:                  map[string]PrincipalController{},
		userControllers:                       map[string]UserController{},
		authConfigControllers:                 map[string]AuthConfigController{},
		tokenControllers:                      map[string]TokenController{},
		dynamicSchemaControllers:              map[string]DynamicSchemaController{},
		preferenceControllers:                 map[string]PreferenceController{},
		clusterLoggingControllers:             map[string]ClusterLoggingController{},
		projectLoggingControllers:             map[string]ProjectLoggingController{},
		listenConfigControllers:               map[string]ListenConfigController{},
		settingControllers:                    map[string]SettingController{},
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

type MachinesGetter interface {
	Machines(namespace string) MachineInterface
}

func (c *Client) Machines(namespace string) MachineInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &MachineResource, MachineGroupVersionKind, machineFactory{})
	return &machineClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type MachineDriversGetter interface {
	MachineDrivers(namespace string) MachineDriverInterface
}

func (c *Client) MachineDrivers(namespace string) MachineDriverInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &MachineDriverResource, MachineDriverGroupVersionKind, machineDriverFactory{})
	return &machineDriverClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type MachineTemplatesGetter interface {
	MachineTemplates(namespace string) MachineTemplateInterface
}

func (c *Client) MachineTemplates(namespace string) MachineTemplateInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &MachineTemplateResource, MachineTemplateGroupVersionKind, machineTemplateFactory{})
	return &machineTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ProjectsGetter interface {
	Projects(namespace string) ProjectInterface
}

func (c *Client) Projects(namespace string) ProjectInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ProjectResource, ProjectGroupVersionKind, projectFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &GlobalRoleResource, GlobalRoleGroupVersionKind, globalRoleFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &GlobalRoleBindingResource, GlobalRoleBindingGroupVersionKind, globalRoleBindingFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &RoleTemplateResource, RoleTemplateGroupVersionKind, roleTemplateFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &PodSecurityPolicyTemplateResource, PodSecurityPolicyTemplateGroupVersionKind, podSecurityPolicyTemplateFactory{})
	return &podSecurityPolicyTemplateClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterRoleTemplateBindingsGetter interface {
	ClusterRoleTemplateBindings(namespace string) ClusterRoleTemplateBindingInterface
}

func (c *Client) ClusterRoleTemplateBindings(namespace string) ClusterRoleTemplateBindingInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterRoleTemplateBindingResource, ClusterRoleTemplateBindingGroupVersionKind, clusterRoleTemplateBindingFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ProjectRoleTemplateBindingResource, ProjectRoleTemplateBindingGroupVersionKind, projectRoleTemplateBindingFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterResource, ClusterGroupVersionKind, clusterFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterEventResource, ClusterEventGroupVersionKind, clusterEventFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterRegistrationTokenResource, ClusterRegistrationTokenGroupVersionKind, clusterRegistrationTokenFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &CatalogResource, CatalogGroupVersionKind, catalogFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &TemplateResource, TemplateGroupVersionKind, templateFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &TemplateVersionResource, TemplateVersionGroupVersionKind, templateVersionFactory{})
	return &templateVersionClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type GroupsGetter interface {
	Groups(namespace string) GroupInterface
}

func (c *Client) Groups(namespace string) GroupInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &GroupResource, GroupGroupVersionKind, groupFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &GroupMemberResource, GroupMemberGroupVersionKind, groupMemberFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &PrincipalResource, PrincipalGroupVersionKind, principalFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &UserResource, UserGroupVersionKind, userFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &AuthConfigResource, AuthConfigGroupVersionKind, authConfigFactory{})
	return &authConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type TokensGetter interface {
	Tokens(namespace string) TokenInterface
}

func (c *Client) Tokens(namespace string) TokenInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &TokenResource, TokenGroupVersionKind, tokenFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &DynamicSchemaResource, DynamicSchemaGroupVersionKind, dynamicSchemaFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &PreferenceResource, PreferenceGroupVersionKind, preferenceFactory{})
	return &preferenceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ClusterLoggingsGetter interface {
	ClusterLoggings(namespace string) ClusterLoggingInterface
}

func (c *Client) ClusterLoggings(namespace string) ClusterLoggingInterface {
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ClusterLoggingResource, ClusterLoggingGroupVersionKind, clusterLoggingFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ProjectLoggingResource, ProjectLoggingGroupVersionKind, projectLoggingFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &ListenConfigResource, ListenConfigGroupVersionKind, listenConfigFactory{})
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
	objectClient := clientbase.NewObjectClient(namespace, c.restClient, &SettingResource, SettingGroupVersionKind, settingFactory{})
	return &settingClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
