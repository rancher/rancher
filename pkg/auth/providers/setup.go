package providers

import (
	"context"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

var authConfigTypes = []string{client.GithubConfigType, client.LocalConfigType}

func SetupAuthConfig(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) {
	Configure(ctx, management)
	schema := schemas.Schema(&managementschema.Version, client.GithubConfigType)
	gp, _ := GetProvider("github")
	schema.Formatter = github.ConfigFormatter
	schema.ActionHandler = gp.ConfigActionHandler

	authConfigBaseSchema := schemas.Schema(&managementschema.Version, client.AuthConfigType)
	for _, authConfigSubtype := range authConfigTypes {
		subSchema := schemas.Schema(&managementschema.Version, authConfigSubtype)
		subSchema.Store = subtype.NewSubTypeStore(authConfigSubtype, authConfigBaseSchema.Store)
	}
}
