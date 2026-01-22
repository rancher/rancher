package resourcequota

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	nsByProjectIndex = "resourcequota.cluster.cattle.io/ns-by-project"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		return registerDeferred(ctx, cluster)
	})

	projects := cluster.Management.Management.Projects(cluster.ClusterName)
	projects.AddHandler(ctx, "resourcequota-deferred", func(key string, obj *v3.Project) (runtime.Object, error) {
		if obj != nil &&
			(obj.Spec.ResourceQuota != nil ||
				obj.Spec.ContainerDefaultResourceLimit != nil ||
				obj.Spec.NamespaceDefaultResourceQuota != nil) {
			return obj, starter()
		}
		return obj, nil
	})

}

func registerDeferred(ctx context.Context, cluster *config.UserContext) error {
	// Index for looking up Namespaces by projectID annotation
	nsInformer := cluster.Corew.Namespace().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsByProjectID,
	}
	if err := nsInformer.AddIndexers(nsIndexers); err != nil {
		return err
	}
	sync := &SyncController{
		Namespaces:          cluster.Corew.Namespace(),
		NsIndexer:           nsInformer.GetIndexer(),
		ResourceQuotas:      cluster.Corew.ResourceQuota(),
		ResourceQuotaLister: cluster.Corew.ResourceQuota().Cache(),
		LimitRange:          cluster.Corew.LimitRange(),
		LimitRangeLister:    cluster.Corew.LimitRange().Cache(),
		ProjectCache:        cluster.Management.Wrangler.Mgmt.Project().Cache(),
	}
	cluster.Corew.Namespace().OnChange(ctx, "resourceQuotaSyncController", sync.syncResourceQuota)

	reconcile := &reconcileController{
		namespaces: cluster.Corew.Namespace(),
		nsIndexer:  nsInformer.GetIndexer(),
		projects:   cluster.Management.Wrangler.Mgmt.Project(),
	}

	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler(ctx, "resourceQuotaNamespacesReconcileController", reconcile.reconcileNamespaces)

	calculate := &calculateLimitController{
		nsIndexer:   nsInformer.GetIndexer(),
		projects:    cluster.Management.Wrangler.Mgmt.Project(),
		namespaces:  cluster.Corew.Namespace(),
		clusterName: cluster.ClusterName,
	}
	cluster.Corew.Namespace().OnChange(ctx, "resourceQuotaUsedLimitController", calculate.calculateResourceQuotaUsed)
	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler(ctx, "resourceQuotaProjectUsedLimitController", calculate.calculateResourceQuotaUsedProject)

	reset := &quotaResetController{
		nsIndexer:  nsInformer.GetIndexer(),
		namespaces: cluster.Corew.Namespace(),
	}
	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler(ctx, "namespaceResourceQuotaResetController", reset.resetNamespaceQuota)

	return nil
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
