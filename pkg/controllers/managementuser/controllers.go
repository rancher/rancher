package managementuser

import (
	"context"
	"fmt"

	"github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	"github.com/rancher/rancher/pkg/controllers/managementuser/cavalidator"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken"
	"github.com/rancher/rancher/pkg/controllers/managementuser/healthsyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/machinerole"
	"github.com/rancher/rancher/pkg/controllers/managementuser/networkpolicy"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nsserviceaccount"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rkecontrolplanecondition"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	"github.com/rancher/rancher/pkg/controllers/managementuser/windows"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/controllers/upgrade.cattle.io"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	if err := rbac.Register(ctx, cluster); err != nil {
		return err
	}
	healthsyncer.Register(ctx, cluster)
	networkpolicy.Register(ctx, cluster)

	secret.Register(ctx, mgmt, cluster, clusterRec)
	resourcequota.Register(ctx, cluster)
	windows.Register(ctx, clusterRec, cluster)
	nsserviceaccount.Register(ctx, cluster)

	// For the local cluster, register nodesyncer immediately without waiting for CAPI.
	// The nodesyncer can work without CAPI for the local cluster since
	// isClusterRestoring() is skipped for local clusters (see nodessyncer.go:reconcileAll).
	// For other clusters, we still need to wait for CAPI to be ready because
	// registerProvV2 requires CAPI resources.
	if cluster.ClusterName == "local" {
		_ = cluster.DeferredStart(ctx, func(ctx context.Context) error {
			nodesyncer.Register(ctx, cluster, nil, kubeConfigGetter)
			return nil
		})()
	}

	mgmt.Wrangler.DeferredCAPIRegistration.DeferFunc(func(capi *wrangler.CAPIContext) {
		_ = cluster.DeferredStart(ctx, func(ctx context.Context) error {
			// For non-local clusters, register nodesyncer with CAPI context
			if cluster.ClusterName != "local" {
				nodesyncer.Register(ctx, cluster, capi, kubeConfigGetter)
			}
			registerProvV2(ctx, cluster, capi, clusterRec)
			return nil
		})()
	})

	registerCaches(cluster)

	// early request an impersonator for initializing it
	if _, err := impersonation.ForCluster(cluster); err != nil {
		return fmt.Errorf("unable to create impersonator for cluster %q: %w", cluster.ClusterName, err)
	}

	if err := cavalidator.Register(ctx, cluster); err != nil {
		return err
	}

	// register controller for API
	cluster.APIAggregation.APIServices("").Controller()

	if clusterRec.Spec.LocalClusterAuthEndpoint.Enabled {
		err := clusterauthtoken.CRDSetup(ctx, cluster.RESTConfig, cluster.Management.Schemas)
		if err != nil {
			return err
		}
		clusterauthtoken.Register(ctx, cluster)
	}

	return managementuserlegacy.Register(ctx, mgmt, cluster, clusterRec, kubeConfigGetter)
}

func registerProvV2(ctx context.Context, cluster *config.UserContext, capi *wrangler.CAPIContext, clusterRec *apimgmtv3.Cluster) {
	if !features.RKE2.Enabled() {
		return
	}

	// Just register the snapshot controller if the cluster is administrated by rancher.
	if clusterRec.Annotations["provisioning.cattle.io/administrated"] == "true" {
		if features.Provisioningv2ETCDSnapshotBackPopulation.Enabled() {
			cluster.K3s = k3s.New(cluster.ControllerFactory)
			snapshotbackpopulate.Register(ctx, cluster, capi)
		}
		cluster.Plan = upgrade.New(cluster.ControllerFactory)
		rkecontrolplanecondition.Register(ctx,
			cluster.ClusterName,
			cluster.Management.Wrangler.Provisioning.Cluster().Cache(),
			cluster.Catalog.V1().App(),
			cluster.Plan.V1().Plan(),
			cluster.Management.Wrangler.RKE.RKEControlPlane())
	}
	machinerole.Register(ctx, cluster)
}

func RegisterFollower(cluster *config.UserContext) error {
	registerCaches(cluster)

	// early request an impersonator for initializing it
	if _, err := impersonation.ForCluster(cluster); err != nil {
		return fmt.Errorf("unable to create impersonator for cluster %q: %w", cluster.ClusterName, err)
	}
	return nil
}

// registerCaches initializes caches early in the initialization process to have them available as soon as possible (instead of on demand when Lister/Cache or Controller are called)
func registerCaches(cluster *config.UserContext) {
	cluster.Corew.Namespace().Informer()
	cluster.RBACw.ClusterRoleBinding().Informer()
	cluster.RBACw.ClusterRole().Informer()
	cluster.RBACw.RoleBinding().Informer()
	cluster.RBACw.Role().Informer()
}

// PreBootstrap is a list of functions that _need_ to be run before the rest of the controllers start
// the functions should return an error if they fail, and the start of the controllers will be blocked until all of them succeed
func PreBootstrap(ctx context.Context, mgmt *config.ScaledContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster, kubeConfigGetter common.KubeConfigGetter) error {
	if cluster.ClusterName == "local" {
		return nil
	}

	err := secret.Bootstrap(ctx, mgmt, cluster, clusterRec)
	if err != nil {
		return fmt.Errorf("failed to bootstrap secrets: %w", err)
	}

	return nil
}
