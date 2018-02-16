package user

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/user/alert"
	"github.com/rancher/rancher/pkg/controllers/user/authz"
	"github.com/rancher/rancher/pkg/controllers/user/dnsrecord"
	"github.com/rancher/rancher/pkg/controllers/user/endpoints"
	"github.com/rancher/rancher/pkg/controllers/user/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/logging"
	"github.com/rancher/rancher/pkg/controllers/user/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/controllers/user/secret"
	"github.com/rancher/rancher/pkg/controllers/user/workloadservice"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) error {
	nodesyncer.Register(cluster)
	healthsyncer.Register(ctx, cluster)
	authz.Register(cluster)
	secret.Register(cluster)
	helm.Register(cluster)
	logging.Register(cluster)
	alert.Register(ctx, cluster)
	nslabels.Register(cluster)
	networkpolicy.Register(cluster)

	userOnlyContext := cluster.UserOnlyContext()
	dnsrecord.Register(ctx, userOnlyContext)
	workloadservice.Register(ctx, userOnlyContext)
	endpoints.Register(ctx, userOnlyContext)

	return nil
}
