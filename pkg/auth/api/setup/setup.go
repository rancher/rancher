package setup

import (
	"context"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	publicSchema "github.com/rancher/types/apis/management.cattle.io/v3public/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3public "github.com/rancher/types/client/management/v3public"
)

var (
	crdVersions = []*types.APIVersion{
		&managementSchema.Version,
	}
)

func Schemas(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	Token(schemas)

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

	return crdStore.AddSchemas(ctx, crdSchemas...)
}

var authProviderTypes = []string{v3public.LocalProviderType, v3public.GithubProviderType}

func AuthProviderSchemas(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	schema := schemas.Schema(&publicSchema.PublicVersion, v3public.AuthProviderType)
	setAuthProvidersStore(schema, management)
	schema.ActionHandler = ActionHandler
	schema.Formatter = AuthProviderFormatter

	for _, apSubtype := range authProviderTypes {
		subSchema := schemas.Schema(&publicSchema.PublicVersion, apSubtype)
		subSchema.Store = subtype.NewSubTypeStore(apSubtype, schema.Store)
		subSchema.ActionHandler = ActionHandler
		subSchema.Formatter = AuthProviderFormatter
	}

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

	return crdStore.AddSchemas(ctx, crdSchemas...)
}

func Token(schemas *types.Schemas) {
	schema := schemas.Schema(&managementSchema.Version, client.TokenType)
	schema.CollectionActions = map[string]types.Action{
		"login": {
			Input:  "loginInput",
			Output: "token",
		},
		"logout": {},
	}
	schema.ActionHandler = tokens.TokenActionHandler
	schema.ListHandler = tokens.TokenListHandler
	schema.CreateHandler = tokens.TokenCreateHandler
	schema.DeleteHandler = tokens.TokenDeleteHandler
}
