package api

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/api/user"
	"github.com/rancher/rancher/pkg/auth/principals"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/requests"
	managementschema "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Setup(ctx context.Context, clusterRouter requests.ClusterRouter, scaledContext *config.ScaledContext, schemas *types.Schemas) {
	principals.Schema(ctx, clusterRouter, scaledContext, schemas)
	providers.SetupAuthConfig(ctx, scaledContext, schemas)
	user.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType), scaledContext)
}

func NewNormanServer(ctx context.Context, clusterRouter requests.ClusterRouter, scaledContext *config.ScaledContext) (http.Handler, error) {
	schemas, err := newSchemas(ctx, scaledContext)
	if err != nil {
		return nil, err
	}

	Setup(ctx, clusterRouter, scaledContext, schemas)

	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	return server, nil
}

func newSchemas(ctx context.Context, apiContext *config.ScaledContext) (*types.Schemas, error) {
	schemas := types.NewSchemas()
	schemas.AddSchemas(managementschema.Schemas)

	factory := &crd.Factory{ClientGetter: apiContext.ClientGetter}
	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, schemas, &managementschema.Version,
		client.AuthConfigType,
		client.GroupMemberType,
		client.GroupType,
		client.TokenType,
		client.UserAttributeType,
		client.UserType,
	)

	return schemas, factory.BatchWait()
}
