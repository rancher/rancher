package managementstored

import (
	"context"
	"sync"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/customization/app"
	"github.com/rancher/rancher/pkg/api/customization/authn"
	"github.com/rancher/rancher/pkg/api/customization/catalog"
	apicluster "github.com/rancher/rancher/pkg/api/customization/cluster"
	"github.com/rancher/rancher/pkg/api/customization/machine"
	"github.com/rancher/rancher/pkg/api/customization/setting"
	"github.com/rancher/rancher/pkg/api/store/cert"
	"github.com/rancher/rancher/pkg/api/store/cluster"
	"github.com/rancher/rancher/pkg/api/store/preference"
	"github.com/rancher/rancher/pkg/api/store/scoped"
	"github.com/rancher/rancher/pkg/auth/principals"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/machineconfig"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

func Setup(ctx context.Context, management *config.ManagementContext) error {
	// Here we setup all types that will be stored in the Management cluster
	schemas := management.Schemas

	wg := &sync.WaitGroup{}
	factory := &crd.Factory{ClientGetter: management.ClientGetter}

	createCrd(ctx, wg, factory, schemas, &managementschema.Version,
		client.AuthConfigType,
		client.CatalogType,
		client.ClusterEventType,
		client.ClusterLoggingType,
		client.ClusterRegistrationTokenType,
		client.ClusterRoleTemplateBindingType,
		client.ClusterType,
		client.DynamicSchemaType,
		client.GlobalRoleBindingType,
		client.GlobalRoleType,
		client.GroupMemberType,
		client.GroupType,
		client.ListenConfigType,
		client.MachineType,
		client.MachineDriverType,
		client.MachineTemplateType,
		client.PodSecurityPolicyTemplateType,
		client.PreferenceType,
		client.ProjectLoggingType,
		client.ProjectRoleTemplateBindingType,
		client.ProjectType,
		client.RoleTemplateType,
		client.SettingType,
		client.TemplateType,
		client.TemplateVersionType,
		client.TokenType,
		client.UserType)
	createCrd(ctx, wg, factory, schemas, &projectschema.Version,
		projectclient.AppType)

	wg.Wait()

	Templates(schemas)
	TemplateVersion(schemas)
	User(schemas, management)
	Catalog(schemas)
	SecretTypes(schemas, management)
	App(schemas, management)
	Setting(schemas)
	ClusterTypes(schemas)
	Preference(schemas, management)
	ClusterRegistrationTokens(schemas)

	if err := MachineTypes(schemas, management); err != nil {
		return err
	}

	principals.Schema(ctx, management, schemas)
	providers.SetupAuthConfig(ctx, management, schemas)
	authn.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType), management)

	setupScopedTypes(schemas)

	return nil
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

func Templates(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateType)
	schema.Formatter = catalog.TemplateFormatter
	schema.LinkHandler = catalog.TemplateIconHandler
}

func TemplateVersion(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateVersionType)
	schema.Formatter = catalog.TemplateVersionFormatter
	schema.LinkHandler = catalog.TemplateVersionReadmeHandler
}

func Catalog(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.CatalogType)
	schema.ResourceActions = map[string]types.Action{
		"refresh": {},
	}
	schema.Formatter = catalog.Formatter
	schema.ActionHandler = catalog.RefreshActionHandler
}

func ClusterRegistrationTokens(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterRegistrationTokenType)
	schema.Store = &cluster.RegistrationTokenStore{
		Store: schema.Store,
	}
	schema.Formatter = func(request *types.APIContext, resource *types.RawResource) {
		token, _ := resource.Values["token"].(string)
		if token != "" {
			url := request.URLBuilder.RelativeToRoot("/" + token + ".yaml")
			resource.Values["command"] = "kubectl apply -f " + url
			resource.Values["token"] = token
			resource.Values["manifestUrl"] = url
		}
	}
}

func SecretTypes(schemas *types.Schemas, management *config.ManagementContext) {
	secretSchema := schemas.Schema(&projectschema.Version, projectclient.SecretType)
	secretSchema.Store = proxy.NewProxyStore(management.ClientGetter,
		config.ManagementStorageContext,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets")

	for _, subSchema := range schemas.SchemasForVersion(projectschema.Version) {
		if subSchema.BaseType == projectclient.SecretType && subSchema.ID != projectclient.SecretType {
			if subSchema.CanList(nil) {
				subSchema.Store = subtype.NewSubTypeStore(subSchema.ID, secretSchema.Store)
			}
		}
	}

	secretSchema = schemas.Schema(&projectschema.Version, projectclient.CertificateType)
	secretSchema.Store = cert.Wrap(secretSchema.Store)
}

func User(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&managementschema.Version, client.UserType)
	schema.Formatter = authn.UserFormatter
	schema.CollectionFormatter = authn.CollectionFormatter
	handler := &authn.Handler{
		UserClient: management.Management.Users(""),
	}
	schema.ActionHandler = handler.Actions
}

func Preference(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&managementschema.Version, client.PreferenceType)
	schema.Store = preference.NewStore(management.Core.Namespaces(""), schema.Store)
}

func MachineTypes(schemas *types.Schemas, management *config.ManagementContext) error {
	secretStore, err := machineconfig.NewStore(management)
	if err != nil {
		return err
	}

	schema := schemas.Schema(&managementschema.Version, client.MachineDriverType)
	machineDriverHandlers := &machine.DriverHandlers{
		MachineDriverClient: management.Management.MachineDrivers(""),
	}
	schema.Formatter = machineDriverHandlers.Formatter
	schema.ActionHandler = machineDriverHandlers.ActionHandler

	machineHandler := &machine.Handler{
		SecretStore: secretStore,
	}

	schema = schemas.Schema(&managementschema.Version, client.MachineType)
	schema.Formatter = machine.Formatter
	schema.LinkHandler = machineHandler.LinkHandler

	schema = schemas.Schema(&managementschema.Version, client.MachineConfigType)

	return nil
}

func App(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&projectschema.Version, projectclient.AppType)
	actionWrapper := app.ActionWrapper{
		Management: *management,
	}
	schema.Formatter = app.Formatter
	schema.ActionHandler = actionWrapper.ActionHandler
}

func Setting(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.SettingType)
	schema.Formatter = setting.Formatter
}

func ClusterTypes(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterType)
	schema.Validator = apicluster.Validator
}
