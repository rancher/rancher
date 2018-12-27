package clusterauthtoken

import (
	"context"

	"github.com/rancher/norman/store/crd"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/client/cluster/v3"
	"github.com/rancher/types/config"
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
	factory.BatchWait()
	return nil
}
