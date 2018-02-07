package user

import (
	"context"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/rancher/pkg/controllers/user/dnsrecord"
	"github.com/rancher/rancher/pkg/controllers/user/endpoints"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/controllers/user/workloadservice"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

func registerUserOnly(ctx context.Context, userOnlyContext *config.UserOnlyContext) error {
	factory, err := crd.NewFactoryFromConfig(userOnlyContext.RESTConfig)
	if err != nil {
		return err
	}

	if _, err := factory.AddSchemas(ctx, userOnlyContext.Schemas.Schema(&schema.Version, client.WorkloadType)); err != nil {
		return err
	}

	workload.Register(ctx, userOnlyContext)
	dnsrecord.Register(ctx, userOnlyContext)
	workloadservice.Register(ctx, userOnlyContext)
	endpoints.Register(ctx, userOnlyContext)

	return nil
}
