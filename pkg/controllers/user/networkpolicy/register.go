package networkpolicy

import (
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

// Register initializes the controllers and registers
func Register(cluster *config.UserContext) {
	logrus.Infof("Registering project network policy")

	pnpLister := cluster.Management.Management.ProjectNetworkPolicies("").Controller().Lister()
	pnps := cluster.Management.Management.ProjectNetworkPolicies("")
	projectLister := cluster.Management.Management.Projects("").Controller().Lister()
	projects := cluster.Management.Management.Projects("")
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
	clusterNetPolHandler := &clusterHandler{cluster, pnpLister, podLister,
		serviceLister, projectLister, clusters, pnps, npmgr, cluster.ClusterName}

	projects.Controller().AddClusterScopedHandler("projectSyncer", cluster.ClusterName, ps.Sync)
	pnps.AddClusterScopedHandler("projectNetworkPolicySyncer", cluster.ClusterName, pnpsyncer.Sync)
	nses.AddHandler("namespaceLifecycle", nss.Sync)
	pods.AddHandler("podHandler", podHandler.Sync)
	services.AddHandler("serviceHandler", serviceHandler.Sync)

	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler("nodeHandler", nodeHandler.Sync)
	clusters.AddHandler("clusterNetPolHandler", clusterNetPolHandler.Sync)
}
