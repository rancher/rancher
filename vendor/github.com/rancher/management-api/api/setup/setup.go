package setup

import (
	"context"

	"net/http"

	"encoding/base64"

	"github.com/rancher/management-api/api/authn"
	"github.com/rancher/management-api/api/catalog"
	"github.com/rancher/management-api/api/project"
	"github.com/rancher/management-api/api/subscribe"
	clustermanager "github.com/rancher/management-api/cluster"
	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/satori/uuid"
)

var crdVersions = []*types.APIVersion{
	&managementschema.Version,
}

func Schemas(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	Subscribe(schemas)
	ProjectLinks(schemas, management)
	Templates(schemas)
	ClusterRegistrationTokens(schemas)
	addUserAction(schemas)

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

	authn.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType))

	return nil
}

func Templates(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateType)
	schema.Formatter = catalog.TemplateFormatter
	schema.LinkHandler = catalog.TemplateIconHandler
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

func Subscribe(schemas *types.Schemas) {
	schemas.MustImportAndCustomize(&builtin.Version, subscribe.Subscribe{}, func(schema *types.Schema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{}
		schema.ListHandler = subscribe.Handler
		schema.PluralName = "subscribe"
	})
}

func addUserAction(schemas *types.Schemas) {
	schemas.MustImport(&managementschema.Version, authn.ChangePasswordInput{})
	schema := schemas.Schema(&managementschema.Version, client.UserType)
	schema.ResourceActions = map[string]types.Action{
		"changepassword": {
			Input:  "changePasswordInput",
			Output: "user",
		},
	}
	schema.Formatter = authn.UserFormatter
	schema.ActionHandler = authn.UserActionHandler
}
