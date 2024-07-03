package managementuser

import (
	"context"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	"github.com/rancher/rancher/pkg/controllers/managementuser/certsexpiration"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/machinerole"
	"github.com/rancher/rancher/pkg/controllers/managementuser/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nsserviceaccount"
	"github.com/rancher/rancher/pkg/controllers/managementuser/pspdelete"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	"github.com/rancher/rancher/pkg/controllers/managementuser/windows"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Register(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	rbac.Register(ctx, cluster)
	healthsyncer.Register(ctx, cluster)
	networkpolicy.Register(ctx, cluster)
	nodesyncer.Register(ctx, cluster, kubeConfigGetter)
	podsecuritypolicy.Register(ctx, cluster)
	secret.Register(ctx, cluster)
	resourcequota.Register(ctx, cluster)
	certsexpiration.Register(ctx, cluster)
	windows.Register(ctx, clusterRec, cluster)
	nsserviceaccount.Register(ctx, cluster)
	if features.RKE2.Enabled() {
		// Just register the snapshot controller if the cluster is administrated by rancher.
		if clusterRec.Annotations["provisioning.cattle.io/administrated"] == "true" {
			snapshotbackpopulate.Register(ctx, cluster)
		}
		pspdelete.Register(ctx, cluster)
		machinerole.Register(ctx, cluster)
	}

	// register controller for API
	cluster.APIAggregation.APIServices("").Controller()
	// register secrets controller for impersonation
	cluster.Core.Secrets("").Controller()

	if clusterRec.Spec.LocalClusterAuthEndpoint.Enabled {
		err := clusterauthtoken.CRDSetup(ctx, cluster.UserOnlyContext())
		if err != nil {
			return err
		}
		clusterauthtoken.Register(ctx, cluster)
	}

	// Ensure these caches are started
	cluster.Core.Namespaces("").Controller()
	cluster.Core.Secrets("").Controller()
	cluster.Core.ServiceAccounts("").Controller()

	return managementuserlegacy.Register(ctx, mgmt, cluster, clusterRec, kubeConfigGetter)
}

func RegisterFollower(cluster *config.UserContext) error {
	cluster.KindNamespaces[schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}] = impersonation.ImpersonationNamespace
	cluster.KindNamespaces[schema.GroupVersionKind{
		Version: "v1",
		Kind:    "ServiceAccount",
	}] = impersonation.ImpersonationNamespace

	cluster.Core.Namespaces("").Controller()
	cluster.Core.Secrets("").Controller()
	cluster.Core.ServiceAccounts("").Controller()
	cluster.RBAC.ClusterRoleBindings("").Controller()
	cluster.RBAC.ClusterRoles("").Controller()
	cluster.RBAC.RoleBindings("").Controller()
	cluster.RBAC.Roles("").Controller()
	return nil
}
