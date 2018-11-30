package providers

import (
	"context"
	"github.com/rancher/rancher/pkg/api/store/auth"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

var authConfigTypes = []string{
	client.GithubConfigType,
	client.LocalConfigType,
	client.ActiveDirectoryConfigType,
	client.AzureADConfigType,
	client.OpenLdapConfigType,
	client.FreeIpaConfigType,
	client.PingConfigType,
	client.ADFSConfigType,
	client.KeyCloakConfigType,
}

func SetupAuthConfig(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) {
	Configure(ctx, management)

	authConfigBaseSchema := schemas.Schema(&managementschema.Version, client.AuthConfigType)
	authConfigBaseSchema.Store = auth.Wrap(authConfigBaseSchema.Store, management.Core.Secrets("mgmt-secrets"))
	for _, authConfigSubtype := range authConfigTypes {
		subSchema := schemas.Schema(&managementschema.Version, authConfigSubtype)
		GetProviderByType(authConfigSubtype).CustomizeSchema(subSchema)
		subSchema.Store = subtype.NewSubTypeStore(authConfigSubtype, authConfigBaseSchema.Store)
	}
}
