package setup

import (
	"context"

	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/management/api/app"
	"github.com/rancher/rancher/pkg/api/management/api/authn"
	"github.com/rancher/rancher/pkg/api/management/api/catalog"
	apicluster "github.com/rancher/rancher/pkg/api/management/api/cluster"
	"github.com/rancher/rancher/pkg/api/management/api/machine"
	"github.com/rancher/rancher/pkg/api/management/api/project"
	"github.com/rancher/rancher/pkg/api/management/api/setting"
	clustermanager "github.com/rancher/rancher/pkg/api/management/cluster"
	"github.com/rancher/rancher/pkg/api/management/store/cert"
	"github.com/rancher/rancher/pkg/api/management/store/cluster"
	"github.com/rancher/rancher/pkg/api/management/store/preference"
	"github.com/rancher/rancher/pkg/api/management/store/scoped"
	"github.com/rancher/rancher/pkg/machine/store"
	machineconfig "github.com/rancher/rancher/pkg/machine/store/config"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

var (
	crdVersions = []*types.APIVersion{
		&managementschema.Version,
	}
)

func Schemas(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	subscribe.Register(&builtin.Version, schemas)
	SubAPIs(schemas, management)
	Templates(schemas)
	TemplateVersion(schemas)
	User(schemas, management)
	Catalog(schemas)
	SecretTypes(schemas, management)
	App(schemas, management)
	Setting(schemas)
	ClusterTypes(schemas)

	secretStore, err := machineconfig.NewStore(management)
	if err != nil {
		return err
	}
	MachineTypes(schemas, management, secretStore)

	crdStore, err := crd.NewCRDStoreFromConfig(management.RESTConfig)
	if err != nil {
		return err
	}

	var crdSchemas []*types.Schema
	for _, version := range crdVersions {
		for _, schema := range schemas.SchemasForVersion(*version) {
			crdSchemas = append(crdSchemas, schema)
		}
	}

	if err := crdStore.AddSchemas(ctx, crdSchemas...); err != nil {
		return err
	}

	AuthConfigs(schemas)
	authn.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType), management)
	Preference(schemas, management)
	ClusterRegistrationTokens(schemas)

	NamespacedTypes(schemas)

	return nil
}

func NamespacedTypes(schemas *types.Schemas) {
	for _, version := range crdVersions {
		for _, schema := range schemas.SchemasForVersion(*version) {
			if schema.Scope != types.NamespaceScope || schema.Store == nil {
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

func SubAPIs(schemas *types.Schemas, management *config.ManagementContext) {
	clusterManager := clustermanager.NewManager(management)
	linkHandler := project.NewClusterRouterLinkHandler(clusterManager)

	schema := schemas.Schema(&managementschema.Version, client.ProjectType)
	schema.Formatter = project.ClusterLinks
	schema.LinkHandler = linkHandler
	schema.ListHandler = project.ListHandler

	schema = schemas.Schema(&managementschema.Version, client.ClusterType)
	schema.Formatter = project.ClusterLinks
	schema.LinkHandler = linkHandler
	schema.ListHandler = project.ListHandler
}

func SecretTypes(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&projectschema.Version, projectclient.SecretType)
	schema.Store = scoped.NewScopedStore("projectId", proxy.NewProxyStore(management.UnversionedClient,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets"))

	for _, secretSubType := range config.ProjectTypes {
		if secretSubType != projectclient.SecretType {
			subSchema := schemas.Schema(&projectschema.Version, secretSubType)
			if subSchema.CanList(nil) {
				subSchema.Store = subtype.NewSubTypeStore(secretSubType, schema.Store)
			}
		}
	}

	schema = schemas.Schema(&projectschema.Version, projectclient.CertificateType)
	schema.Store = &cert.Store{
		Store: schema.Store,
	}
}

var authConfigTypes = []string{client.GithubConfigType, client.LocalConfigType}

func AuthConfigs(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.GithubConfigType)
	schema.Formatter = authn.GithubConfigFormatter
	schema.ActionHandler = authn.GithubConfigActionHandler

	authConfigBaseSchema := schemas.Schema(&managementschema.Version, client.AuthConfigType)
	for _, authConfigSubtype := range authConfigTypes {
		subSchema := schemas.Schema(&managementschema.Version, authConfigSubtype)
		subSchema.Store = subtype.NewSubTypeStore(authConfigSubtype, authConfigBaseSchema.Store)
	}
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

func MachineTypes(schemas *types.Schemas, management *config.ManagementContext, secretStore *store.GenericEncryptedStore) {
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

}

func App(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&managementschema.Version, client.AppType)
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
