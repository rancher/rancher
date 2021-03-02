package managementuser

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/managementagent"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	"github.com/rancher/rancher/pkg/controllers/managementuser/certsexpiration"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nsserviceaccount"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	"github.com/rancher/rancher/pkg/controllers/managementuser/settings"
	"github.com/rancher/rancher/pkg/controllers/managementuser/windows"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy"
	"github.com/rancher/rancher/pkg/features"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext, clusterRec *managementv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	rbac.Register(ctx, cluster)
	healthsyncer.Register(ctx, cluster)
	networkpolicy.Register(ctx, cluster)
	nodesyncer.Register(ctx, cluster, kubeConfigGetter)
	podsecuritypolicy.RegisterCluster(ctx, cluster)
	podsecuritypolicy.RegisterClusterRole(ctx, cluster)
	podsecuritypolicy.RegisterBindings(ctx, cluster)
	podsecuritypolicy.RegisterNamespace(ctx, cluster)
	podsecuritypolicy.RegisterPodSecurityPolicy(ctx, cluster)
	podsecuritypolicy.RegisterServiceAccount(ctx, cluster)
	podsecuritypolicy.RegisterTemplate(ctx, cluster)
	secret.Register(ctx, cluster)
	resourcequota.Register(ctx, cluster)
	certsexpiration.Register(ctx, cluster)
	windows.Register(ctx, clusterRec, cluster)
	nsserviceaccount.Register(ctx, cluster)

	// register controller for API
	cluster.APIAggregation.APIServices("").Controller()

	if clusterRec.Spec.LocalClusterAuthEndpoint.Enabled {
		err := clusterauthtoken.CRDSetup(ctx, cluster.UserOnlyContext())
		if err != nil {
			return err
		}
		clusterauthtoken.Register(ctx, cluster)
	}

	if clusterRec.Spec.Internal {
		err := managementagent.Register(ctx, cluster.UserOnlyContext())
		if err != nil {
			return err
		}
	} else {
		err := settings.Register(ctx, cluster)
		if err != nil {
			return err
		}
	}

	if features.Legacy.Enabled() {
		if err := managementuserlegacy.Register(ctx, cluster, clusterRec, kubeConfigGetter); err != nil {
			return err
		}
	}

	return nil
}

func RegisterFollower(ctx context.Context, cluster *config.UserContext, kubeConfigGetter common.KubeConfigGetter, clusterManager healthsyncer.ClusterControllerLifecycle) error {
	cluster.Core.Pods("").Controller()
	cluster.Core.Namespaces("").Controller()
	cluster.Core.Services("").Controller()
	cluster.RBAC.ClusterRoleBindings("").Controller()
	cluster.RBAC.RoleBindings("").Controller()
	cluster.Core.Endpoints("").Controller()
	cluster.APIAggregation.APIServices("").Controller()
	return nil
}
