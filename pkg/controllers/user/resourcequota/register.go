package resourcequota

import (
	"context"

	"github.com/rancher/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	nsByProjectIndex = "resourcequota.cluster.cattle.io/ns-by-project"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	// Index for looking up Namespaces by projectID annotation
	nsInformer := cluster.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsByProjectID,
	}
	nsInformer.AddIndexers(nsIndexers)
	sync := &SyncController{
		Namespaces:          cluster.Core.Namespaces(""),
		NamespaceLister:     cluster.Core.Namespaces("").Controller().Lister(),
		NsIndexer:           nsInformer.GetIndexer(),
		ResourceQuotas:      cluster.Core.ResourceQuotas(""),
		ResourceQuotaLister: cluster.Core.ResourceQuotas("").Controller().Lister(),
		ProjectLister:       cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
	}
	cluster.Core.Namespaces("").AddHandler("resourceQuotaSyncController", sync.syncResourceQuota)

	reconcile := &reconcileController{
		namespaces: cluster.Core.Namespaces(""),
		nsIndexer:  nsInformer.GetIndexer(),
	}

	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler("resourceQuotaNamespacesReconcileController", reconcile.reconcileNamespaces)

	cleanup := &cleanupController{
		projectLister:       cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		namespaceLister:     cluster.Core.Namespaces("").Controller().Lister(),
		resourceQuotas:      cluster.Core.ResourceQuotas(""),
		resourceQuotaLister: cluster.Core.ResourceQuotas("").Controller().Lister(),
	}
	cluster.Core.ResourceQuotas("").AddHandler("resourceQuotaCleanupController", cleanup.cleanup)

	calculate := &calculateLimitController{
		nsIndexer:     nsInformer.GetIndexer(),
		projectLister: cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		projects:      cluster.Management.Management.Projects(cluster.ClusterName),
		clusterName:   cluster.ClusterName,
	}
	cluster.Core.Namespaces("").AddHandler("resourceQuotaUsedLimitController", calculate.calculateResourceQuotaUsed)
	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler("resourceQuotaProjectUsedLimitController", calculate.calculateResourceQuotaUsedProject)

	reset := &quotaResetController{
		nsIndexer:  nsInformer.GetIndexer(),
		namespaces: cluster.Core.Namespaces(""),
	}
	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler("namespaceResourceQuotaResetController", reset.resetNamespaceQuota)
}

func nsByProjectID(obj interface{}) ([]string, error) {
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		return []string{}, nil
	}

	if id, ok := ns.Annotations[projectIDAnnotation]; ok {
		return []string{id}, nil
	}

	return []string{}, nil
}
