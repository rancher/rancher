package user

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/rancher/pkg/controllers/user/alert"
	"github.com/rancher/rancher/pkg/controllers/user/approuter"
	"github.com/rancher/rancher/pkg/controllers/user/dnsrecord"
	"github.com/rancher/rancher/pkg/controllers/user/endpoints"
	"github.com/rancher/rancher/pkg/controllers/user/externalservice"
	"github.com/rancher/rancher/pkg/controllers/user/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/ingress"
	"github.com/rancher/rancher/pkg/controllers/user/ingresshostgen"
	"github.com/rancher/rancher/pkg/controllers/user/logging"
	"github.com/rancher/rancher/pkg/controllers/user/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/user/noderemove"
	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline"
	"github.com/rancher/rancher/pkg/controllers/user/rbac"
	"github.com/rancher/rancher/pkg/controllers/user/rbac/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/user/resourcequota"
	"github.com/rancher/rancher/pkg/controllers/user/secret"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/controllers/user/targetworkloadservice"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/config"
	// init upgrade implement
	_ "github.com/rancher/rancher/pkg/controllers/user/alert/upgrade"
	_ "github.com/rancher/rancher/pkg/controllers/user/logging/upgrade"
	_ "github.com/rancher/rancher/pkg/controllers/user/pipeline/upgrade"
)

func Register(ctx context.Context, cluster *config.UserContext, kubeConfigGetter common.KubeConfigGetter, clusterManager healthsyncer.ClusterControllerLifecycle) error {
	alert.Register(ctx, cluster)
	rbac.Register(cluster)
	healthsyncer.Register(ctx, cluster, clusterManager)
	helm.Register(cluster, kubeConfigGetter)
	logging.Register(ctx, cluster)
	networkpolicy.Register(cluster)
	noderemove.Register(cluster)
	nodesyncer.Register(ctx, cluster, kubeConfigGetter)
	nslabels.Register(cluster)
	pipeline.Register(ctx, cluster)
	podsecuritypolicy.RegisterCluster(cluster)
	podsecuritypolicy.RegisterBindings(cluster)
	podsecuritypolicy.RegisterNamespace(cluster)
	podsecuritypolicy.RegisterServiceAccount(cluster)
	podsecuritypolicy.RegisterTemplate(cluster)
	secret.Register(cluster)
	systemimage.Register(ctx, cluster)
	endpoints.Register(ctx, cluster)
	approuter.Register(ctx, cluster)
	resourcequota.Register(ctx, cluster)

	userOnlyContext := cluster.UserOnlyContext()
	dnsrecord.Register(ctx, userOnlyContext)
	externalservice.Register(ctx, userOnlyContext)
	ingress.Register(ctx, userOnlyContext, cluster)
	ingresshostgen.Register(userOnlyContext)
	targetworkloadservice.Register(ctx, userOnlyContext)
	workload.Register(ctx, userOnlyContext)

	return nil
}
