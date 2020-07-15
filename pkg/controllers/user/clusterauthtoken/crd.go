package clusterauthtoken

import (
	"context"

	"github.com/rancher/norman/store/crd"
	client "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func CRDSetup(ctx context.Context, apiContext *config.UserOnlyContext) error {
	factory, err := crd.NewFactoryFromClient(apiContext.RESTConfig)
	if err != nil {
		return err
	}
	factory.BatchCreateCRDs(ctx, config.UserStorageContext, apiContext.Schemas, &clusterSchema.Version,
		client.ClusterAuthTokenType,
		client.ClusterUserAttributeType,
	)
	return factory.BatchWait()
}
