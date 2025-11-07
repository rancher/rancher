package resourcequota

import (
	"fmt"
	"reflect"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
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
	projects   wmgmtv3.ProjectClient
}

func (r *reconcileController) reconcileNamespaces(_ string, p *apiv3.Project) (runtime.Object, error) {
	if p == nil || p.DeletionTimestamp != nil {
		return nil, nil
	}
	projectID := fmt.Sprintf("%s:%s", p.Namespace, p.Name)
	namespaces, err := r.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return nil, err
	}

	// With no namespaces used-limit has to be empty because there is
	// nothing which can be used without namespaces. Therefore squash
	// non-empty used-limits, if present.
	if len(namespaces) == 0 &&
		p.Spec.ResourceQuota != nil &&
		!isEmpty(&p.Spec.ResourceQuota.UsedLimit) {

		logrus.Warnf("project %q, clearing bogus used-limit", p.Name)

		newP := p.DeepCopy()
		newP.Spec.ResourceQuota.UsedLimit = apiv3.ResourceQuotaLimit{}
		_, err := r.projects.Update(newP)
		if err != nil {
			logrus.Errorf("project %q, clearing bogus used-limit failed: %q", p.Name, err)
			return nil, err
		}
	}

	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		r.namespaces.Enqueue(ns.Name)
	}
	return nil, nil
}

func isEmpty(rql *apiv3.ResourceQuotaLimit) bool {
	return reflect.ValueOf(rql).IsZero()
}
