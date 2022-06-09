package networkpolicy

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})
	clusters := cluster.Management.Management.Clusters("")
	clusters.AddHandler(ctx, "networkpolicy-deferred", func(key string, obj *v3.Cluster) (runtime.Object, error) {
		if obj != nil &&
			obj.Name == cluster.ClusterName &&
			obj.Spec.EnableNetworkPolicy != nil &&
			*obj.Spec.EnableNetworkPolicy {
			return obj, starter()
		}
		return obj, nil
	})
}

func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	logrus.Infof("Registering project network policy")

	pnpLister := cluster.Management.Management.ProjectNetworkPolicies("").Controller().Lister()
	pnps := cluster.Management.Management.ProjectNetworkPolicies("")
	projectLister := cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister()
	projects := cluster.Management.Management.Projects(cluster.ClusterName)
	clusterLister := cluster.Management.Management.Clusters("").Controller().Lister()
	mgmtClusters := cluster.Management.Management.Clusters("")
	clusters := cluster.Management.Wrangler.Provisioning.Cluster().Cache()

	nodeLister := cluster.Core.Nodes("").Controller().Lister()
	nsLister := cluster.Core.Namespaces("").Controller().Lister()
	nses := cluster.Core.Namespaces("")
	serviceLister := cluster.Core.Services("").Controller().Lister()
	services := cluster.Core.Services("")
	podLister := cluster.Core.Pods("").Controller().Lister()
	pods := cluster.Core.Pods("")

	npLister := cluster.Networking.NetworkPolicies("").Controller().Lister()
	npClient := cluster.Networking

	npmgr := &netpolMgr{clusterLister, clusters, nsLister, nodeLister, pods, projects,
		npLister, npClient, projectLister, cluster.ClusterName}
	ps := &projectSyncer{pnpLister, pnps, projects, clusterLister, cluster.ClusterName}
	nss := &nsSyncer{npmgr, clusterLister, serviceLister, podLister,
		services, pods, cluster.ClusterName}
	pnpsyncer := &projectNetworkPolicySyncer{npmgr}
	podHandler := &podHandler{npmgr, pods, clusterLister, cluster.ClusterName}
	serviceHandler := &serviceHandler{npmgr, clusterLister, cluster.ClusterName}
	nodeHandler := &nodeHandler{npmgr, clusterLister, cluster.ClusterName}
	clusterHandler := &clusterHandler{cluster, pnpLister, podLister,
		serviceLister, projectLister, mgmtClusters, pnps, npmgr, cluster.ClusterName}

	clusterNetAnnHandler := &clusterNetAnnHandler{mgmtClusters, cluster.ClusterName}

	projects.Controller().AddClusterScopedHandler(ctx, "projectSyncer", cluster.ClusterName, ps.Sync)
	pnps.AddClusterScopedHandler(ctx, "projectNetworkPolicySyncer", cluster.ClusterName, pnpsyncer.Sync)
	nses.AddHandler(ctx, "namespaceLifecycle", nss.Sync)
	pods.AddHandler(ctx, "podHandler", podHandler.Sync)
	services.AddHandler(ctx, "serviceHandler", serviceHandler.Sync)

	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "nodeHandler", nodeHandler.Sync)
	mgmtClusters.AddHandler(ctx, "clusterHandler", clusterHandler.Sync)

	mgmtClusters.AddHandler(ctx, "clusterNetAnnHandler", clusterNetAnnHandler.Sync)
}
