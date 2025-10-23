package clusterauthtoken

import (
	"context"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	client "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/client-go/rest"
)

func CRDSetup(ctx context.Context, restConfig rest.Config, schemas *types.Schemas) error {
	factory, err := crd.NewFactoryFromClient(restConfig)
	if err != nil {
		return err
	}
	factory.BatchCreateCRDs(ctx, config.UserStorageContext, scheme.Scheme, schemas, &clusterSchema.Version,
		client.ClusterAuthTokenType,
		client.ClusterUserAttributeType,
	)
	return factory.BatchWait()
}
