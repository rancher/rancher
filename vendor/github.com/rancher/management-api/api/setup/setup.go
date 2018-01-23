package setup

import (
	"context"

	"encoding/base64"

	"github.com/rancher/management-api/api/authn"
	"github.com/rancher/management-api/api/catalog"
	"github.com/rancher/management-api/api/cluster"
	"github.com/rancher/management-api/api/machine"
	"github.com/rancher/management-api/api/project"
	"github.com/rancher/management-api/api/stack"
	clustermanager "github.com/rancher/management-api/cluster"
	"github.com/rancher/management-api/store/cert"
	"github.com/rancher/management-api/store/preference"
	"github.com/rancher/management-api/store/scoped"
	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/satori/uuid"
)

var (
	crdVersions = []*types.APIVersion{
		&managementschema.Version,
	}
)

func Schemas(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	subscribe.Register(&builtin.Version, schemas)
	ProjectLinks(schemas, management)
	Templates(schemas)
	TemplateVersion(schemas)
	ClusterRegistrationTokens(schemas)
	User(schemas, management)
	Catalog(schemas)
	SecretTypes(schemas, management)
	MachineTypes(schemas, management)
	ClusterTypes(schemas)
	Stack(schemas, management)

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

	authn.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType), management)
	Preference(schemas, management)

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
	schema.Formatter = func(request *types.APIContext, resource *types.RawResource) {
		token := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(uuid.NewV4().Bytes())
		url := request.URLBuilder.RelativeToRoot("/" + token + ".yaml")
		resource.Values["command"] = "kubectl apply -f " + url
		resource.Values["token"] = token
		resource.Values["manifestUrl"] = url
	}
}

func ProjectLinks(schemas *types.Schemas, management *config.ManagementContext) {
	clusterManager := clustermanager.NewManager(management)
	linkHandler := project.NewClusterRouterLinkHandler(clusterManager)

	schema := schemas.Schema(&managementschema.Version, client.ProjectType)
	schema.Formatter = project.ClusterLinks
	schema.LinkHandler = linkHandler

	schema = schemas.Schema(&managementschema.Version, client.ClusterType)
	schema.Formatter = project.ClusterLinks
	schema.LinkHandler = linkHandler
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

func MachineTypes(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&managementschema.Version, client.MachineDriverType)
	handlers := &machine.Handlers{
		MachineDriverClient: management.Management.MachineDrivers(""),
	}
	schema.Formatter = handlers.Formatter
	schema.ActionHandler = handlers.ActionHandler

	schema = schemas.Schema(&managementschema.Version, client.MachineType)
	schema.Validator = machine.Validator
	schema.Formatter = machine.Formatter

	schema = schemas.Schema(&managementschema.Version, client.MachineConfigType)
	schema.Validator = machine.Validator

}

func ClusterTypes(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterType)
	schema.Validator = cluster.Validator
}

func Stack(schemas *types.Schemas, management *config.ManagementContext) {
	schema := schemas.Schema(&managementschema.Version, client.StackType)
	actionWrapper := stack.ActionWrapper{
		Management: *management,
	}
	schema.ActionHandler = actionWrapper.ActionHandler
}
