package resourcequota

import (
	"fmt"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcache "k8s.io/client-go/tools/cache"
)

/*
quotaResetController is responsible for resetting resource quota on the namespace
when project resource quota gets reset
*/
type quotaResetController struct {
	namespaces v1.NamespaceInterface
	nsIndexer  clientcache.Indexer
}

func (c *quotaResetController) resetNamespaceQuota(key string, project *v3.Project) (runtime.Object, error) {
	if project == nil || project.DeletionTimestamp != nil {
		return nil, nil
	}
	if project.Spec.ResourceQuota != nil {
		return nil, nil
	}
	projectID := fmt.Sprintf("%s:%s", project.Namespace, project.Name)
	namespaces, err := c.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return nil, err
	}
	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		quota := getNamespaceResourceQuota(ns)
		if quota == "" {
			continue
		}
		toUpdate := ns.DeepCopy()
		delete(toUpdate.Annotations, resourceQuotaAnnotation)
		if _, err := c.namespaces.Update(toUpdate); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
