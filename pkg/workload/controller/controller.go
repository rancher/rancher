package controller

import (
	"context"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/workload-controller/controller/dnsrecord"
	"github.com/rancher/workload-controller/controller/workload"
	"github.com/rancher/workload-controller/controller/workloadservice"
)

func Register(ctx context.Context, workloadContext *config.WorkloadContext) error {
	factory, err := crd.NewFactoryFromConfig(workloadContext.RESTConfig)
	if err != nil {
		return err
	}

	if _, err := factory.AddSchemas(ctx, workloadContext.Schemas.Schema(&schema.Version, client.WorkloadType)); err != nil {
		return err
	}

	workload.Register(ctx, workloadContext)

	dnsrecord.Register(ctx, workloadContext)
	workloadservice.Register(ctx, workloadContext)

	return nil
}
