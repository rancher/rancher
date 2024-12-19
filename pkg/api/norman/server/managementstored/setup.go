package managementstored

import (
	"context"
	"net/http"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/norman/customization/alert"
	"github.com/rancher/rancher/pkg/api/norman/customization/app"
	"github.com/rancher/rancher/pkg/api/norman/customization/authn"
	"github.com/rancher/rancher/pkg/api/norman/customization/catalog"
	ccluster "github.com/rancher/rancher/pkg/api/norman/customization/cluster"
	"github.com/rancher/rancher/pkg/api/norman/customization/clustertemplate"
	"github.com/rancher/rancher/pkg/api/norman/customization/cred"
	"github.com/rancher/rancher/pkg/api/norman/customization/etcdbackup"
	"github.com/rancher/rancher/pkg/api/norman/customization/feature"
	"github.com/rancher/rancher/pkg/api/norman/customization/globaldns"
	"github.com/rancher/rancher/pkg/api/norman/customization/globalrole"
	"github.com/rancher/rancher/pkg/api/norman/customization/globalrolebinding"
	"github.com/rancher/rancher/pkg/api/norman/customization/kontainerdriver"
	"github.com/rancher/rancher/pkg/api/norman/customization/monitor"
	"github.com/rancher/rancher/pkg/api/norman/customization/multiclusterapp"
	"github.com/rancher/rancher/pkg/api/norman/customization/namespacedresource"
	"github.com/rancher/rancher/pkg/api/norman/customization/node"
	"github.com/rancher/rancher/pkg/api/norman/customization/nodepool"
	"github.com/rancher/rancher/pkg/api/norman/customization/nodetemplate"
	psptBinding "github.com/rancher/rancher/pkg/api/norman/customization/podsecuritypolicybinding"
	"github.com/rancher/rancher/pkg/api/norman/customization/podsecuritypolicytemplate"
	projectaction "github.com/rancher/rancher/pkg/api/norman/customization/project"
	"github.com/rancher/rancher/pkg/api/norman/customization/roletemplate"
	"github.com/rancher/rancher/pkg/api/norman/customization/roletemplatebinding"
	"github.com/rancher/rancher/pkg/api/norman/customization/secret"
	"github.com/rancher/rancher/pkg/api/norman/customization/setting"
	alertStore "github.com/rancher/rancher/pkg/api/norman/store/alert"
	appStore "github.com/rancher/rancher/pkg/api/norman/store/app"
	catalogStore "github.com/rancher/rancher/pkg/api/norman/store/catalog"
	"github.com/rancher/rancher/pkg/api/norman/store/cert"
	"github.com/rancher/rancher/pkg/api/norman/store/cluster"
	clustertemplatestore "github.com/rancher/rancher/pkg/api/norman/store/clustertemplate"
	featStore "github.com/rancher/rancher/pkg/api/norman/store/feature"
	globaldnsAPIStore "github.com/rancher/rancher/pkg/api/norman/store/globaldns"
	globalRoleStore "github.com/rancher/rancher/pkg/api/norman/store/globalrole"
	grbstore "github.com/rancher/rancher/pkg/api/norman/store/globalrolebindings"
	nodeStore "github.com/rancher/rancher/pkg/api/norman/store/node"
	nodeTemplateStore "github.com/rancher/rancher/pkg/api/norman/store/nodetemplate"
	"github.com/rancher/rancher/pkg/api/norman/store/noopwatching"
	passwordStore "github.com/rancher/rancher/pkg/api/norman/store/password"
	"github.com/rancher/rancher/pkg/api/norman/store/preference"
	rtStore "github.com/rancher/rancher/pkg/api/norman/store/roletemplate"
	"github.com/rancher/rancher/pkg/api/norman/store/scoped"
	settingstore "github.com/rancher/rancher/pkg/api/norman/store/setting"
	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/pkg/auth/api"
	authapi "github.com/rancher/rancher/pkg/auth/api"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	projectclient "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/clusterrouter"
	md "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/nodeconfig"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	projectschema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Setup(ctx context.Context, apiContext *config.ScaledContext, clusterManager *clustermanager.Manager,
	k8sProxy http.Handler, localClusterEnabled bool) error {
	// Here we setup all types that will be stored in the Management cluster
	schemas := apiContext.Schemas

	factory := &crd.Factory{ClientGetter: apiContext.ClientGetter}

	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, scheme.Scheme, schemas, &managementschema.Version,
		client.AuthConfigType,
		client.ClusterRegistrationTokenType,
		client.ClusterRoleTemplateBindingType,
		client.ClusterType,
		client.DynamicSchemaType,
		client.EtcdBackupType,
		client.FeatureType,
		client.FleetWorkspaceType,
		client.GlobalRoleBindingType,
		client.GlobalRoleType,
		client.GroupMemberType,
		client.GroupType,
		client.KontainerDriverType,
		client.NodeDriverType,
		client.NodePoolType,
		client.NodeTemplateType,
		client.NodeType,
		client.PodSecurityAdmissionConfigurationTemplateType,
		client.PodSecurityPolicyTemplateProjectBindingType,
		client.PodSecurityPolicyTemplateType,
		client.PreferenceType,
		client.ProjectNetworkPolicyType,
		client.ProjectRoleTemplateBindingType,
		client.ProjectType,
		client.RkeK8sSystemImageType,
		client.RkeK8sServiceOptionType,
		client.RkeAddonType,
		client.RoleTemplateType,
		client.SamlTokenType,
		client.SettingType,
		client.TokenType,
		client.UserAttributeType,
		client.UserType,
		client.ClusterTemplateType,
		client.ClusterTemplateRevisionType,
	)

	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, scheme.Scheme, schemas, &managementschema.Version,
		client.CatalogType,
		client.CatalogTemplateType,
		client.CatalogTemplateVersionType,
		client.ClusterAlertType,
		client.ClusterAlertGroupType,
		client.ClusterCatalogType,
		client.ClusterAlertRuleType,
		client.ClusterMonitorGraphType,
		client.ComposeConfigType,
		client.MultiClusterAppType,
		client.MultiClusterAppRevisionType,
		client.MonitorMetricType,
		client.NotifierType,
		client.ProjectAlertType,
		client.ProjectAlertGroupType,
		client.ProjectCatalogType,
		client.ProjectAlertRuleType,
		client.ProjectMonitorGraphType,
		client.TemplateType,
		client.TemplateVersionType,
		client.TemplateContentType,
		client.GlobalDnsType,
		client.GlobalDnsProviderType,
		client.RancherUserNotificationType,
	)

	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, scheme.Scheme, schemas, &projectschema.Version,
		projectclient.AppType,
		projectclient.AppRevisionType,
	)

	if err := factory.BatchWait(); err != nil {
		return err
	}

	Clusters(ctx, schemas, apiContext, clusterManager, k8sProxy)
	ClusterRoleTemplateBinding(schemas, apiContext)
	api.User(ctx, schemas, apiContext)
	SecretTypes(ctx, schemas, apiContext)
	Setting(schemas)
	Feature(schemas, apiContext)
	Preference(schemas, apiContext)
	ClusterRegistrationTokens(schemas, apiContext)
	Tokens(ctx, schemas, apiContext)
	NodeTemplates(schemas, apiContext)
	Project(schemas, apiContext)
	ProjectRoleTemplateBinding(schemas, apiContext)
	PodSecurityAdmissionConfigurationTemplate(schemas, apiContext)
	PodSecurityPolicyTemplate(schemas, apiContext)
	PodSecurityPolicyTemplateProjectBinding(schemas, apiContext)
	GlobalRole(schemas, apiContext)
	GlobalRoleBindings(schemas, apiContext)
	RoleTemplate(schemas, apiContext)
	KontainerDriver(schemas, apiContext)
	ClusterTemplates(schemas, apiContext)
	SystemImages(schemas, apiContext)
	EtcdBackups(schemas, apiContext)
	RancherUserNotifications(schemas, apiContext)

	Templates(ctx, schemas, apiContext)
	TemplateVersion(ctx, schemas, apiContext)
	Catalog(schemas, apiContext)
	ProjectCatalog(schemas, apiContext)
	ClusterCatalog(schemas, apiContext)
	App(schemas, apiContext, clusterManager)
	Alert(schemas, apiContext)
	TemplateContent(schemas)
	Monitor(schemas, apiContext, clusterManager)
	MultiClusterApps(schemas, apiContext)
	GlobalDNSs(schemas, apiContext, localClusterEnabled)
	GlobalDNSProviders(schemas, apiContext, localClusterEnabled)

	if err := NodeTypes(schemas, apiContext); err != nil {
		return err
	}

	authapi.Setup(ctx, clusterrouter.GetClusterID, apiContext, schemas)
	authn.SetRTBStore(ctx, schemas.Schema(&managementschema.Version, client.ClusterRoleTemplateBindingType), apiContext)
	authn.SetRTBStore(ctx, schemas.Schema(&managementschema.Version, client.ProjectRoleTemplateBindingType), apiContext)
	nodeStore.SetupStore(schemas.Schema(&managementschema.Version, client.NodeType))
	projectaction.SetProjectStore(schemas.Schema(&managementschema.Version, client.ProjectType), apiContext)
	setupScopedTypes(schemas)
	setupPasswordTypes(ctx, schemas, apiContext)

	multiclusterapp.SetMemberStore(ctx, schemas.Schema(&managementschema.Version, client.MultiClusterAppType), apiContext)
	GlobalDNSProvidersPwdWrap(schemas, apiContext, localClusterEnabled)

	return nil
}

func setupPasswordTypes(ctx context.Context, schemas *types.Schemas, management *config.ScaledContext) {
	secretStore := management.Core.Secrets("")
	nsStore := management.Core.Namespaces("")
	passwordStore.SetPasswordStore(schemas, secretStore, nsStore)
}

func setupScopedTypes(schemas *types.Schemas) {
	for _, schema := range schemas.Schemas() {
		if schema.Scope != types.NamespaceScope || schema.Store == nil || schema.Store.Context() != config.ManagementStorageContext {
			continue
		}

		for _, key := range []string{"projectId", "clusterId"} {
			ns, ok := schema.ResourceFields["namespaceId"]
			if !ok {
				continue
			}

			if _, ok := schema.ResourceFields[key]; !ok {
				continue
			}

			schema.Store = scoped.NewScopedStore(key, schema.Store)
			ns.Required = false
			schema.ResourceFields["namespaceId"] = ns
			break
		}
	}
}

func Clusters(ctx context.Context, schemas *types.Schemas, managementContext *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterType)
	clusterFormatter := ccluster.NewFormatter(schemas, managementContext)
	schema.Formatter = clusterFormatter.Formatter
	schema.CollectionFormatter = clusterFormatter.CollectionFormatter
	clusterStore := cluster.GetClusterStore(schema, managementContext, clusterManager, k8sProxy)
	schema.Store = clusterStore

	handler := ccluster.ActionHandler{
		NodepoolGetter:                managementContext.Management,
		NodeLister:                    managementContext.Management.Nodes("").Controller().Lister(),
		ClusterClient:                 managementContext.Management.Clusters(""),
		CatalogManager:                managementContext.CatalogManager,
		UserMgr:                       managementContext.UserManager,
		ClusterManager:                clusterManager,
		NodeTemplateGetter:            managementContext.Management,
		BackupClient:                  managementContext.Management.EtcdBackups(""),
		ClusterTemplateClient:         managementContext.Management.ClusterTemplates(""),
		ClusterTemplateRevisionClient: managementContext.Management.ClusterTemplateRevisions(""),
		SubjectAccessReviewClient:     managementContext.K8sClient.AuthorizationV1().SubjectAccessReviews(),
		TokenClient:                   managementContext.Management.Tokens(""),
		Auth:                          requests.NewAuthenticator(ctx, clusterrouter.GetClusterID, managementContext),
	}

	clusterValidator := ccluster.Validator{
		ClusterClient:                 managementContext.Management.Clusters(""),
		ClusterLister:                 managementContext.Management.Clusters("").Controller().Lister(),
		ClusterTemplateLister:         managementContext.Management.ClusterTemplates("").Controller().Lister(),
		ClusterTemplateRevisionLister: managementContext.Management.ClusterTemplateRevisions("").Controller().Lister(),
		Users:                         managementContext.Management.Users(""),
		GrbLister:                     managementContext.Management.GlobalRoleBindings("").Controller().Lister(),
		GrLister:                      managementContext.Management.GlobalRoles("").Controller().Lister(),
	}

	handler.CatalogTemplateVersionLister = managementContext.Management.CatalogTemplateVersions("").Controller().Lister()
	schema.ActionHandler = handler.ClusterActionHandler
	schema.Validator = clusterValidator.Validator
}

func Templates(ctx context.Context, schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateType)
	schema.Scope = types.NamespaceScope
	schema.Store = catalog.GetTemplateStore(ctx, managementContext)

	wrapper := catalog.TemplateWrapper{
		CatalogLister:                managementContext.Management.Catalogs("").Controller().Lister(),
		ClusterCatalogLister:         managementContext.Management.ClusterCatalogs("").Controller().Lister(),
		ProjectCatalogLister:         managementContext.Management.ProjectCatalogs("").Controller().Lister(),
		CatalogTemplateVersionLister: managementContext.Management.CatalogTemplateVersions("").Controller().Lister(),
		SecretLister:                 managementContext.Core.Secrets("").Controller().Lister(),
	}
	schema.Formatter = wrapper.TemplateFormatter
	schema.LinkHandler = wrapper.TemplateIconHandler

	schemaCatalogTemplate := schemas.Schema(&managementschema.Version, client.CatalogTemplateType)
	schemaCatalogTemplate.CollectionMethods = []string{}
	schemaCatalogTemplate.ResourceMethods = []string{}
}

func TemplateVersion(ctx context.Context, schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateVersionType)
	schema.Scope = types.NamespaceScope
	schema.Store = proxy.NewProxyStore(ctx, managementContext.ClientGetter,
		config.ManagementStorageContext,
		scheme.Scheme,
		[]string{"apis"},
		"management.cattle.io",
		"v3",
		"CatalogTemplateVersion",
		"catalogtemplateversions")
	t := catalog.TemplateVerionFormatterWrapper{
		CatalogLister:        managementContext.Management.Catalogs("").Controller().Lister(),
		ClusterCatalogLister: managementContext.Management.ClusterCatalogs("").Controller().Lister(),
		ProjectCatalogLister: managementContext.Management.ProjectCatalogs("").Controller().Lister(),
		SecretLister:         managementContext.Core.Secrets("").Controller().Lister(),
	}
	schema.Formatter = t.TemplateVersionFormatter
	schema.LinkHandler = t.TemplateVersionReadmeHandler
	schema.Store = noopwatching.Wrap(schema.Store)

	schemaCatalogTemplateVersion := schemas.Schema(&managementschema.Version, client.CatalogTemplateVersionType)
	schemaCatalogTemplateVersion.CollectionMethods = []string{}
	schemaCatalogTemplateVersion.ResourceMethods = []string{}
}

func TemplateContent(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateContentType)
	schema.Store = noopwatching.Wrap(schema.Store)
	schema.CollectionMethods = []string{}
}

func Catalog(schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.CatalogType)
	schema.Formatter = catalog.Formatter
	handler := catalog.ActionHandler{
		CatalogClient: managementContext.Management.Catalogs(""),
	}
	schema.ActionHandler = handler.RefreshActionHandler
	schema.CollectionFormatter = catalog.CollectionFormatter
	schema.LinkHandler = handler.ExportYamlHandler
	schema.Validator = catalog.Validator
	users := managementContext.Management.Users("")
	grbLister := managementContext.Management.GlobalRoleBindings("").Controller().Lister()
	grLister := managementContext.Management.GlobalRoles("").Controller().Lister()
	secretLister := managementContext.Core.Secrets("").Controller().Lister()
	secrets := managementContext.Core.Secrets("")
	clusterLister := managementContext.Management.Clusters("").Controller().Lister()
	schema.Store = catalogStore.Wrap(schema.Store, managementContext, users, grbLister, grLister, secretLister, secrets, clusterLister)
}

func ProjectCatalog(schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ProjectCatalogType)
	schema.Formatter = catalog.Formatter
	handler := catalog.ActionHandler{
		ProjectCatalogClient: managementContext.Management.ProjectCatalogs(""),
	}
	schema.ActionHandler = handler.RefreshProjectCatalogActionHandler
	schema.CollectionFormatter = catalog.CollectionFormatter
	schema.Validator = catalog.Validator
	users := managementContext.Management.Users("")
	grbLister := managementContext.Management.GlobalRoleBindings("").Controller().Lister()
	grLister := managementContext.Management.GlobalRoles("").Controller().Lister()
	secretLister := managementContext.Core.Secrets("").Controller().Lister()
	secrets := managementContext.Core.Secrets("")
	clusterLister := managementContext.Management.Clusters("").Controller().Lister()
	schema.Store = catalogStore.Wrap(schema.Store, managementContext, users, grbLister, grLister, secretLister, secrets, clusterLister)
}

func ClusterCatalog(schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterCatalogType)
	schema.Formatter = catalog.Formatter
	handler := catalog.ActionHandler{
		ClusterCatalogClient: managementContext.Management.ClusterCatalogs(""),
	}
	schema.ActionHandler = handler.RefreshClusterCatalogActionHandler
	schema.CollectionFormatter = catalog.CollectionFormatter
	schema.Validator = catalog.Validator
	users := managementContext.Management.Users("")
	grbLister := managementContext.Management.GlobalRoleBindings("").Controller().Lister()
	grLister := managementContext.Management.GlobalRoles("").Controller().Lister()
	secretLister := managementContext.Core.Secrets("").Controller().Lister()
	secrets := managementContext.Core.Secrets("")
	clusterLister := managementContext.Management.Clusters("").Controller().Lister()
	schema.Store = catalogStore.Wrap(schema.Store, managementContext, users, grbLister, grLister, secretLister, secrets, clusterLister)
}

func ClusterRegistrationTokens(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterRegistrationTokenType)
	schema.Store = &cluster.RegistrationTokenStore{
		Store: schema.Store,
	}
}

func Tokens(ctx context.Context, schemas *types.Schemas, mgmt *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.TokenType)
	manager := tokens.NewManager(ctx, mgmt)

	schema.Store = &transform.Store{
		Store:             schema.Store,
		StreamTransformer: manager.TokenStreamTransformer,
	}
}

func NodeTemplates(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.NodeTemplateType)
	npl := management.Management.NodePools("").Controller().Lister()
	nl := management.Management.Nodes("").Controller().Lister()
	userLister := management.Management.Users("").Controller().Lister()
	f := nodetemplate.Formatter{
		NodePoolLister: npl,
		NodeLister:     nl,
		UserLister:     userLister,
	}
	schema.Formatter = f.Formatter

	nsLister := management.Core.Namespaces("")
	nodeTemplateGlobalStore := namespacedresource.Wrap(schema.Store, nsLister, namespace.NodeTemplateGlobalNamespace)

	globalSecretLister := management.Core.Secrets(namespace.GlobalNamespace).Controller().Lister()
	nodeTemplateClient := management.Management.NodeTemplates("")

	s := nodeTemplateStore.Wrap(nodeTemplateGlobalStore, npl, nl, globalSecretLister, nodeTemplateClient)
	schema.Store = s
	schema.Validator = nodetemplate.Validator
}

func SecretTypes(ctx context.Context, schemas *types.Schemas, management *config.ScaledContext) {
	secretSchema := schemas.Schema(&projectschema.Version, projectclient.SecretType)
	secretSchema.Store = proxy.NewProxyStore(ctx, management.ClientGetter,
		config.ManagementStorageContext,
		scheme.Scheme,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets")
	secretSchema.Validator = secret.Validator
	for _, subSchema := range schemas.SchemasForVersion(projectschema.Version) {
		if subSchema.BaseType == projectclient.SecretType && subSchema.ID != projectclient.SecretType {
			if subSchema.CanList(nil) == nil {
				subSchema.Store = subtype.NewSubTypeStore(subSchema.ID, secretSchema.Store)
				subSchema.Validator = secret.Validator
			}
		}
	}

	secretSchema = schemas.Schema(&projectschema.Version, projectclient.CertificateType)
	secretSchema.Store = cert.Wrap(secretSchema.Store)

	mgmtSecretSchema := schemas.Schema(&managementschema.Version, client.ManagementSecretType)
	mgmtSecretSchema.Store = proxy.NewProxyStore(ctx, management.ClientGetter,
		config.ManagementStorageContext,
		scheme.Scheme,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets")

	credSchema := schemas.Schema(&managementschema.Version, client.CloudCredentialType)
	credSchema.Store = cred.Wrap(mgmtSecretSchema.Store,
		management.Core.Namespaces(""),
		management.Management.NodeTemplates("").Controller().Lister(),
		management.Wrangler.Provisioning.Cluster().Cache(),
	)
	credSchema.Validator = cred.Validator
}

func Preference(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.PreferenceType)
	schema.Store = preference.NewStore(management.Core.Namespaces(""), schema.Store)
}

func NodeTypes(schemas *types.Schemas, management *config.ScaledContext) error {
	secretStore, err := nodeconfig.NewStore(management.Core.Namespaces(""), management.Core)
	if err != nil {
		return err
	}

	schema := schemas.Schema(&managementschema.Version, client.NodeDriverType)
	machineDriverHandlers := &node.DriverHandlers{
		NodeDriverClient: management.Management.NodeDrivers(""),
	}
	schema.Formatter = machineDriverHandlers.Formatter
	schema.ActionHandler = machineDriverHandlers.ActionHandler
	schema.LinkHandler = machineDriverHandlers.ExportYamlHandler

	machineHandler := &node.Handler{
		SecretStore: secretStore,
	}

	schema = schemas.Schema(&managementschema.Version, client.NodeType)
	schema.Formatter = node.Formatter
	schema.LinkHandler = machineHandler.LinkHandler
	actionWrapper := node.ActionWrapper{}
	schema.ActionHandler = actionWrapper.ActionHandler

	schema = schemas.Schema(&managementschema.Version, client.NodePoolType)
	ntl := management.Management.NodeTemplates("").Controller().Lister()
	f := &nodepool.Formatter{
		NodeTemplateLister: ntl,
	}
	schema.Formatter = f.Formatter

	nodepoolValidator := nodepool.Validator{
		NodePoolLister: management.Management.NodePools("").Controller().Lister(),
	}
	schema.Validator = nodepoolValidator.Validator
	return nil
}

func App(schemas *types.Schemas, management *config.ScaledContext, clusterManager *clustermanager.Manager) {
	schema := schemas.Schema(&projectschema.Version, projectclient.AppType)
	store := &appStore.Store{
		Store:                 schema.Store,
		Apps:                  management.Project.Apps("").Controller().Lister(),
		TemplateVersionLister: management.Management.CatalogTemplateVersions("").Controller().Lister(),
		CatalogManager:        management.CatalogManager,
		ClusterLister:         management.Management.Clusters("").Controller().Lister(),
	}
	schema.Store = store
	wrapper := app.Wrapper{
		Clusters:              management.Management.Clusters(""),
		ClusterManager:        clusterManager,
		CatalogManager:        management.CatalogManager,
		TemplateVersionClient: management.Management.CatalogTemplateVersions(""),
		TemplateVersionLister: management.Management.CatalogTemplateVersions("").Controller().Lister(),
		AppGetter:             management.Project,
		UserLister:            management.Management.Users("").Controller().Lister(),
		UserManager:           management.UserManager,
	}
	schema.Formatter = app.Formatter
	schema.ActionHandler = wrapper.ActionHandler
	schema.LinkHandler = wrapper.LinkHandler
	schema.Validator = wrapper.Validator
}

func Setting(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.SettingType)
	schema.Formatter = setting.Formatter
	schema.Validator = setting.Validator
	schema.Store = settingstore.New(schema.Store)
}

func Feature(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.FeatureType)
	validator := feature.Validator{FeatureLister: management.Management.Features("").Controller().Lister()}
	schema.Validator = validator.Validator
	schema.Formatter = feature.Formatter
	schema.Store = featStore.New(schema.Store)
}

func Alert(schemas *types.Schemas, management *config.ScaledContext) {
	handler := &alert.Handler{
		ClusterAlertRule: management.Management.ClusterAlertRules(""),
		ProjectAlertRule: management.Management.ProjectAlertRules(""),
		Notifiers:        management.Management.Notifiers(""),
		DialerFactory:    management.Dialer,
	}

	schema := schemas.Schema(&managementschema.Version, client.NotifierType)
	schema.CollectionFormatter = alert.NotifierCollectionFormatter
	schema.Formatter = alert.NotifierFormatter
	schema.ActionHandler = handler.NotifierActionHandler
	schema.Store = alertStore.NewNotifier(management, schema.Store)

	schema = schemas.Schema(&managementschema.Version, client.ClusterAlertRuleType)
	schema.Formatter = alert.RuleFormatter
	schema.Validator = alert.ClusterAlertRuleValidator
	schema.ActionHandler = handler.ClusterAlertRuleActionHandler

	schema = schemas.Schema(&managementschema.Version, client.ProjectAlertRuleType)
	schema.Formatter = alert.RuleFormatter
	schema.Validator = alert.ProjectAlertRuleValidator
	schema.ActionHandler = handler.ProjectAlertRuleActionHandler

	//old schema just for migrate
	schema = schemas.Schema(&managementschema.Version, client.ClusterAlertType)
	schema = schemas.Schema(&managementschema.Version, client.ProjectAlertType)
}

func Monitor(schemas *types.Schemas, management *config.ScaledContext, clusterManager *clustermanager.Manager) {
	clusterGraphHandler := monitor.NewClusterGraphHandler(management.Dialer, clusterManager)
	projectGraphHandler := monitor.NewProjectGraphHandler(management.Dialer, clusterManager)
	metricHandler := monitor.NewMetricHandler(management.Dialer, clusterManager)

	schema := schemas.Schema(&managementschema.Version, client.ClusterMonitorGraphType)
	schema.CollectionFormatter = monitor.QueryGraphCollectionFormatter
	schema.ActionHandler = clusterGraphHandler.QuerySeriesAction

	schema = schemas.Schema(&managementschema.Version, client.ProjectMonitorGraphType)
	schema.CollectionFormatter = monitor.QueryGraphCollectionFormatter
	schema.ActionHandler = projectGraphHandler.QuerySeriesAction

	schema = schemas.Schema(&managementschema.Version, client.MonitorMetricType)
	schema.CollectionFormatter = monitor.MetricCollectionFormatter
	schema.ActionHandler = metricHandler.Action
}

func Project(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ProjectType)
	schema.Formatter = projectaction.Formatter
	handler := &projectaction.Handler{
		Projects:                 management.Management.Projects(""),
		ProjectLister:            management.Management.Projects("").Controller().Lister(),
		UserMgr:                  management.UserManager,
		ClusterManager:           management.ClientGetter.(*clustermanager.Manager),
		ClusterLister:            management.Management.Clusters("").Controller().Lister(),
		ProvisioningClusterCache: management.Wrangler.Provisioning.Cluster().Cache(),
		PSPTemplateLister:        management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
	}
	schema.ActionHandler = handler.Actions
}

func PodSecurityAdmissionConfigurationTemplate(schemas *types.Schemas, management *config.ScaledContext) {
	schemas.Schema(&managementschema.Version, client.PodSecurityAdmissionConfigurationTemplateType)
}

func PodSecurityPolicyTemplate(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.PodSecurityPolicyTemplateType)
	schema.Formatter = podsecuritypolicytemplate.NewFormatter(management)
	schema.Store = &podsecuritypolicytemplate.Store{
		Store: schema.Store,
	}
	schema.Validator = podsecuritypolicytemplate.Validator
}

func PodSecurityPolicyTemplateProjectBinding(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.PodSecurityPolicyTemplateProjectBindingType)
	schema.Validator = psptBinding.NewValidator(management)
}

func ClusterRoleTemplateBinding(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterRoleTemplateBindingType)
	schema.Validator = roletemplatebinding.NewCRTBValidator(management)
}

func ProjectRoleTemplateBinding(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ProjectRoleTemplateBindingType)
	schema.Validator = roletemplatebinding.NewPRTBValidator(management)
}

func GlobalRole(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.GlobalRoleType)
	grLister := management.Management.GlobalRoles("").Controller().Lister()
	schema.Store = globalRoleStore.Wrap(schema.Store, grLister)
	schema.Formatter = globalrole.Formatter
	w := globalrole.Wrapper{
		GlobalRoleLister: grLister,
	}
	schema.Validator = w.Validator
}

func GlobalRoleBindings(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.GlobalRoleBindingType)
	grLister := management.Management.GlobalRoles("").Controller().Lister()
	schema.Store = grbstore.Wrap(schema.Store, grLister)
	schema.Validator = globalrolebinding.Validator
}

func RoleTemplate(schemas *types.Schemas, management *config.ScaledContext) {
	rt := roletemplate.Wrapper{
		RoleTemplateLister: management.Management.RoleTemplates("").Controller().Lister(),
	}
	schema := schemas.Schema(&managementschema.Version, client.RoleTemplateType)
	schema.Formatter = rt.Formatter
	schema.Validator = rt.Validator
	schema.Store = rtStore.Wrap(schema.Store, management.Management.RoleTemplates("").Controller().Lister())
}

func KontainerDriver(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.KontainerDriverType)
	metadataHandler := md.MetadataController{
		SystemImagesLister:   management.Management.RkeK8sSystemImages("").Controller().Lister(),
		SystemImages:         management.Management.RkeK8sSystemImages(""),
		ServiceOptionsLister: management.Management.RkeK8sServiceOptions("").Controller().Lister(),
		ServiceOptions:       management.Management.RkeK8sServiceOptions(""),
		AddonsLister:         management.Management.RkeAddons("").Controller().Lister(),
		Addons:               management.Management.RkeAddons(""),
		SettingLister:        management.Management.Settings("").Controller().Lister(),
		Settings:             management.Management.Settings(""),
	}

	handler := kontainerdriver.ActionHandler{
		KontainerDrivers:      management.Management.KontainerDrivers(""),
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
		MetadataHandler:       metadataHandler,
	}
	lh := kontainerdriver.ListHandler{
		SysImageLister:  management.Management.RkeK8sSystemImages("").Controller().Lister(),
		SysImages:       management.Management.RkeK8sSystemImages(""),
		ConfigMapLister: management.Core.ConfigMaps("").Controller().Lister(),
	}
	lh.CatalogLister = management.Management.Catalogs("").Controller().Lister()
	schema.ActionHandler = handler.ActionHandler
	schema.CollectionFormatter = kontainerdriver.CollectionFormatter
	schema.Formatter = kontainerdriver.NewFormatter(management)
	schema.Store = kontainerdriver.NewStore(management, schema.Store)
	schema.ListHandler = lh.ListHandler
	kontainerDriverValidator := kontainerdriver.Validator{
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
	}
	schema.Validator = kontainerDriverValidator.Validator
}

func MultiClusterApps(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.MultiClusterAppType)
	schema.Store = namespacedresource.Wrap(schema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
	revisionSchema := schemas.Schema(&managementschema.Version, client.MultiClusterAppRevisionType)
	revisionSchema.Store = namespacedresource.Wrap(revisionSchema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
	wrapper := multiclusterapp.Wrapper{
		MultiClusterApps:              management.Management.MultiClusterApps(""),
		MultiClusterAppLister:         management.Management.MultiClusterApps("").Controller().Lister(),
		MultiClusterAppRevisionLister: management.Management.MultiClusterAppRevisions("").Controller().Lister(),
		PrtbLister:                    management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		CrtbLister:                    management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		RoleTemplateLister:            management.Management.RoleTemplates("").Controller().Lister(),
		Users:                         management.Management.Users(""),
		GrbLister:                     management.Management.GlobalRoleBindings("").Controller().Lister(),
		GrLister:                      management.Management.GlobalRoles("").Controller().Lister(),
		Prtbs:                         management.Management.ProjectRoleTemplateBindings(""),
		Crtbs:                         management.Management.ClusterRoleTemplateBindings(""),
		ProjectLister:                 management.Management.Projects("").Controller().Lister(),
		ClusterLister:                 management.Management.Clusters("").Controller().Lister(),
		Apps:                          management.Project.Apps(""),
		TemplateVersionLister:         management.Management.CatalogTemplateVersions("").Controller().Lister(),
		CatalogManager:                management.CatalogManager,
	}
	schema.Formatter = wrapper.Formatter
	schema.ActionHandler = wrapper.ActionHandler
	schema.LinkHandler = wrapper.LinkHandler
	schema.Validator = wrapper.Validator
}

func GlobalDNSs(schemas *types.Schemas, management *config.ScaledContext, localClusterEnabled bool) {
	gdns := globaldns.Wrapper{
		GlobalDNSes:           management.Management.GlobalDnses(""),
		GlobalDNSLister:       management.Management.GlobalDnses("").Controller().Lister(),
		PrtbLister:            management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		MultiClusterAppLister: management.Management.MultiClusterApps("").Controller().Lister(),
		Users:                 management.Management.Users(""),
		GrbLister:             management.Management.GlobalRoleBindings("").Controller().Lister(),
		GrLister:              management.Management.GlobalRoles("").Controller().Lister(),
	}
	schema := schemas.Schema(&managementschema.Version, client.GlobalDnsType)
	schema.Store = namespacedresource.Wrap(schema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
	schema.Formatter = gdns.Formatter
	schema.ActionHandler = gdns.ActionHandler
	schema.Validator = gdns.Validator
	schema.Store = globaldnsAPIStore.Wrap(schema.Store)
	if !localClusterEnabled {
		schema.CollectionMethods = []string{}
		schema.ResourceMethods = []string{}
	}
}

func GlobalDNSProviders(schemas *types.Schemas, management *config.ScaledContext, localClusterEnabled bool) {
	schema := schemas.Schema(&managementschema.Version, client.GlobalDnsProviderType)
	schema.Store = namespacedresource.Wrap(schema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
	schema.Store = globaldnsAPIStore.ProviderWrap(schema.Store)
	if !localClusterEnabled {
		schema.CollectionMethods = []string{}
		schema.ResourceMethods = []string{}
	}
}

func GlobalDNSProvidersPwdWrap(schemas *types.Schemas, management *config.ScaledContext, localClusterEnabled bool) {
	schema := schemas.Schema(&managementschema.Version, client.GlobalDnsProviderType)
	schema.Store = globaldnsAPIStore.ProviderPwdWrap(schema.Store)
}

func ClusterTemplates(schemas *types.Schemas, management *config.ScaledContext) {
	wrapper := clustertemplate.Wrapper{
		ClusterTemplates:              management.Management.ClusterTemplates(""),
		ClusterTemplateLister:         management.Management.ClusterTemplates("").Controller().Lister(),
		ClusterTemplateRevisionLister: management.Management.ClusterTemplateRevisions("").Controller().Lister(),
		ClusterTemplateRevisions:      management.Management.ClusterTemplateRevisions(""),
	}
	wrapper.ClusterTemplateQuestions = wrapper.BuildQuestionsFromSchema(schemas.Schema(&managementschema.Version, client.ClusterSpecBaseType), schemas, "")

	schema := schemas.Schema(&managementschema.Version, client.ClusterTemplateType)
	schema.Store = namespacedresource.Wrap(schema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
	schema.Store = clustertemplatestore.WrapStore(schema.Store, management)

	schema.Formatter = wrapper.Formatter
	schema.LinkHandler = wrapper.LinkHandler

	revisionSchema := schemas.Schema(&managementschema.Version, client.ClusterTemplateRevisionType)
	revisionSchema.Store = namespacedresource.Wrap(revisionSchema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
	revisionSchema.Store = clustertemplatestore.WrapStore(revisionSchema.Store, management)
	revisionSchema.Formatter = wrapper.RevisionFormatter
	revisionSchema.CollectionFormatter = wrapper.CollectionFormatter
	revisionSchema.ActionHandler = wrapper.ClusterTemplateRevisionsActionHandler
}

func SystemImages(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.RkeK8sSystemImageType)
	schema.Store = namespacedresource.Wrap(schema.Store, management.Core.Namespaces(""), namespace.GlobalNamespace)
}

func EtcdBackups(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.EtcdBackupType)
	schema.Formatter = etcdbackup.Formatter
}

func RancherUserNotifications(schemas *types.Schemas, management *config.ScaledContext) {
	schemas.Schema(&managementschema.Version, client.RancherUserNotificationType)
}
