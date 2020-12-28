package networkpolicy

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

// Register initializes the controllers and registers
func Register(ctx context.Context, cluster *config.UserContext) {
	logrus.Infof("Registering project network policy")

	pnpLister := cluster.Management.Management.ProjectNetworkPolicies("").Controller().Lister()
	pnps := cluster.Management.Management.ProjectNetworkPolicies("")
	projectLister := cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister()
	projects := cluster.Management.Management.Projects(cluster.ClusterName)
	clusterLister := cluster.Management.Management.Clusters("").Controller().Lister()
	clusters := cluster.Management.Management.Clusters("")

	nodeLister := cluster.Core.Nodes("").Controller().Lister()
	nsLister := cluster.Core.Namespaces("").Controller().Lister()
	nses := cluster.Core.Namespaces("")
	serviceLister := cluster.Core.Services("").Controller().Lister()
	services := cluster.Core.Services("")
	podLister := cluster.Core.Pods("").Controller().Lister()
	pods := cluster.Core.Pods("")

	npLister := cluster.Networking.NetworkPolicies("").Controller().Lister()
	npClient := cluster.Networking

	npmgr := &netpolMgr{nsLister, nodeLister, pods, projects,
		npLister, npClient, projectLister, cluster.ClusterName}
	ps := &projectSyncer{pnpLister, pnps, projects, clusterLister, cluster.ClusterName}
	nss := &nsSyncer{npmgr, clusterLister, serviceLister, podLister,
		services, pods, cluster.ClusterName}
	pnpsyncer := &projectNetworkPolicySyncer{npmgr}
	podHandler := &podHandler{npmgr, pods, clusterLister, cluster.ClusterName}
	serviceHandler := &serviceHandler{npmgr, clusterLister, cluster.ClusterName}
	nodeHandler := &nodeHandler{npmgr, clusterLister, cluster.ClusterName}
	clusterHandler := &clusterHandler{cluster, pnpLister, podLister,
		serviceLister, projectLister, clusters, pnps, npmgr, cluster.ClusterName}

	clusterNetAnnHandler := &clusterNetAnnHandler{clusters, cluster.ClusterName}

	projects.Controller().AddClusterScopedHandler(ctx, "projectSyncer", cluster.ClusterName, ps.Sync)
	pnps.AddClusterScopedHandler(ctx, "projectNetworkPolicySyncer", cluster.ClusterName, pnpsyncer.Sync)
	nses.AddHandler(ctx, "namespaceLifecycle", nss.Sync)
	pods.AddHandler(ctx, "podHandler", podHandler.Sync)
	services.AddHandler(ctx, "serviceHandler", serviceHandler.Sync)

	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "nodeHandler", nodeHandler.Sync)
	clusters.AddHandler(ctx, "clusterHandler", clusterHandler.Sync)

	clusters.AddHandler(ctx, "clusterNetAnnHandler", clusterNetAnnHandler.Sync)
}
