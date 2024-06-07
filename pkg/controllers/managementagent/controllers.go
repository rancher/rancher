package managementagent

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementagent/dnsrecord"
	"github.com/rancher/rancher/pkg/controllers/managementagent/endpoints"
	"github.com/rancher/rancher/pkg/controllers/managementagent/externalservice"
	"github.com/rancher/rancher/pkg/controllers/managementagent/ingress"
	"github.com/rancher/rancher/pkg/controllers/managementagent/ingresshostgen"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nsserviceaccount"
	"github.com/rancher/rancher/pkg/controllers/managementagent/podresources"
	"github.com/rancher/rancher/pkg/controllers/managementagent/targetworkloadservice"
	"github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *config.UserOnlyContext) error {
	dnsrecord.Register(ctx, cluster)
	externalservice.Register(ctx, cluster)
	endpoints.Register(ctx, cluster)
	ingress.Register(ctx, cluster)
	ingresshostgen.Register(ctx, cluster)
	nslabels.Register(ctx, cluster)
	podresources.Register(ctx, cluster)
	targetworkloadservice.Register(ctx, cluster)
	workload.Register(ctx, cluster)
	nsserviceaccount.Register(ctx, cluster)

	return nil
}
