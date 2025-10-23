package resourcequota

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcache "k8s.io/client-go/tools/cache"
)

/*
reconcile controller listens on project updates, and enqueues the namespaces of the project
so they get a chance to reconcile the resource quotas
*/
type reconcileController struct {
	namespaces corew.NamespaceController
	nsIndexer  clientcache.Indexer
}

func (r *reconcileController) reconcileNamespaces(key string, p *v3.Project) (runtime.Object, error) {
	if p == nil || p.DeletionTimestamp != nil {
		return nil, nil
	}
	projectID := fmt.Sprintf("%s:%s", p.Namespace, p.Name)
	namespaces, err := r.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return nil, err
	}

	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		r.namespaces.Enqueue(ns.Name)
	}
	return nil, nil
}
