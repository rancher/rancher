package user

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/user/alert"
	"github.com/rancher/rancher/pkg/controllers/user/authz"
	"github.com/rancher/rancher/pkg/controllers/user/authz/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/user/dnsrecord"
	"github.com/rancher/rancher/pkg/controllers/user/endpoints"
	"github.com/rancher/rancher/pkg/controllers/user/externalservice"
	"github.com/rancher/rancher/pkg/controllers/user/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/ingress"
	"github.com/rancher/rancher/pkg/controllers/user/logging"
	"github.com/rancher/rancher/pkg/controllers/user/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline"
	"github.com/rancher/rancher/pkg/controllers/user/secret"
	"github.com/rancher/rancher/pkg/controllers/user/targetworkloadservice"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
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
	pipeline.Register(ctx, cluster)
	podsecuritypolicy.RegisterServiceAccount(cluster)
	podsecuritypolicy.RegisterTemplate(cluster)
	podsecuritypolicy.RegisterCluster(cluster)
	podsecuritypolicy.RegisterProject(cluster)
	podsecuritypolicy.RegisterNamespace(cluster)
	networkpolicy.Register(cluster)

	userOnlyContext := cluster.UserOnlyContext()
	dnsrecord.Register(ctx, userOnlyContext)
	targetworkloadservice.Register(ctx, userOnlyContext)
	endpoints.Register(ctx, userOnlyContext)
	workload.Register(ctx, userOnlyContext)
	ingress.Register(ctx, userOnlyContext)
	externalservice.Register(ctx, userOnlyContext)

	return nil
}
