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
	sync := &syncController{
		namespaces:                  cluster.Core.Namespaces(""),
		namespaceLister:             cluster.Core.Namespaces("").Controller().Lister(),
		resourceQuotas:              cluster.Core.ResourceQuotas(""),
		resourceQuotaLister:         cluster.Core.ResourceQuotas("").Controller().Lister(),
		resourceQuotaTemplateLister: cluster.Management.Management.ResourceQuotaTemplates(cluster.ClusterName).Controller().Lister(),
		projectLister:               cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
	}
	cluster.Core.Namespaces("").AddHandler("resourceQuotaSyncController", sync.syncResourceQuota)

	// Index for looking up namespaces by projectID annotation
	nsInformer := cluster.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsByProjectID,
	}
	nsInformer.AddIndexers(nsIndexers)
	validate := &validationController{
		namespaces:                  cluster.Core.Namespaces(""),
		nsIndexer:                   nsInformer.GetIndexer(),
		resourceQuotaLister:         cluster.Core.ResourceQuotas("").Controller().Lister(),
		projectLister:               cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		resourceQuotaTemplateLister: cluster.Management.Management.ResourceQuotaTemplates(cluster.ClusterName).Controller().Lister(),
		clusterName:                 cluster.ClusterName,
	}

	cluster.Core.Namespaces("").AddHandler("resourceQuotaValidationController", validate.validateTemplate)

	cleanup := &cleanupController{
		projectLister:       cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		namespaceLister:     cluster.Core.Namespaces("").Controller().Lister(),
		resourceQuotas:      cluster.Core.ResourceQuotas(""),
		resourceQuotaLister: cluster.Core.ResourceQuotas("").Controller().Lister(),
	}
	cluster.Core.ResourceQuotas("").AddHandler("resourceQuotaCleanupController", cleanup.cleanup)

	calculate := &calculateLimitController{
		nsIndexer:                   nsInformer.GetIndexer(),
		resourceQuotaTemplateLister: cluster.Management.Management.ResourceQuotaTemplates(cluster.ClusterName).Controller().Lister(),
		projectLister:               cluster.Management.Management.Projects(cluster.ClusterName).Controller().Lister(),
		projects:                    cluster.Management.Management.Projects(cluster.ClusterName),
		clusterName:                 cluster.ClusterName,
	}
	cluster.Core.Namespaces("").AddHandler("resourceQuotaUsedLimitController", calculate.calculateResourceQuotaUsed)

	reset := &templateResetController{
		nsIndexer:  nsInformer.GetIndexer(),
		namespaces: cluster.Core.Namespaces(""),
	}
	cluster.Management.Management.Projects(cluster.ClusterName).AddHandler("resourceQuotaTemplateResetController", reset.resetTemplate)
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
