package networkpolicy

import (
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func Register(cluster *config.UserContext) {
	logrus.Infof("Registering project network policy")

	pnpLister := cluster.Management.Management.ProjectNetworkPolicies("").Controller().Lister()
	pnpClient := cluster.Management.Management.ProjectNetworkPolicies("").ObjectClient()
	projClient := cluster.Management.Management.Projects("").ObjectClient()
	nsLister := cluster.Core.Namespaces("").Controller().Lister()
	k8sClient := cluster.K8sClient

	npmgr := &netpolMgr{nsLister, k8sClient}
	ps := &projectSyncer{pnpLister, pnpClient, projClient}
	nss := &nsSyncer{npmgr}
	pnps := &projectNetworkPolicySyncer{npmgr}

	cluster.Management.Management.Projects("").Controller().AddClusterScopedHandler("projectSyncer", cluster.ClusterName, ps.Sync)
	cluster.Management.Management.ProjectNetworkPolicies("").AddClusterScopedHandler("projectNetworkPolicySyncer", cluster.ClusterName, pnps.Sync)
	cluster.Core.Namespaces("").AddHandler("namespaceLifecycle", nss.Sync)
}
