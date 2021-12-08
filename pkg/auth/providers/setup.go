package providers

import (
	"context"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/namespace"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
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
	client.OKTAConfigType,
	client.ShibbolethConfigType,
	client.GoogleOauthConfigType,
	client.OIDCConfigType,
	client.KeyCloakOIDCConfigType,
}

func SetupAuthConfig(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) {
	Configure(ctx, management)

	authConfigBaseSchema := schemas.Schema(&managementschema.Version, client.AuthConfigType)
	authConfigBaseSchema.Store = secrets.Wrap(authConfigBaseSchema.Store, management.Core.Secrets(namespace.GlobalNamespace))
	for _, authConfigSubtype := range authConfigTypes {
		subSchema := schemas.Schema(&managementschema.Version, authConfigSubtype)
		GetProviderByType(authConfigSubtype).CustomizeSchema(subSchema)
		subSchema.Store = subtype.NewSubTypeStore(authConfigSubtype, authConfigBaseSchema.Store)
	}
}
