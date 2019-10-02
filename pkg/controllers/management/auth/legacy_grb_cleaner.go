package auth

import (
	"strings"

	"github.com/rancher/norman/lifecycle"
	grbstore "github.com/rancher/rancher/pkg/api/store/globalrolebindings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type grbCleaner struct {
	clusterLister v3.ClusterLister
	mgmt          *config.ManagementContext
}

func newLegacyGRBCleaner(m *config.ManagementContext) *grbCleaner {
	return &grbCleaner{
		mgmt:          m,
		clusterLister: m.Management.Clusters("").Controller().Lister(),
	}
}

// this function addresses issues with grb not being cleaned up that was an issue from v2.2.3 - v2.2.8
// it works on previously created objects outside of the timeline of normal sync handlers to correct issues with finalizers not be removed on deletion and finalizers not being dropped on cluster deletion
func (p *grbCleaner) sync(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}
	if obj.Annotations[grbstore.GrbVersion] == "true" {
		return obj, nil
	}
	obj, err := p.removeFinalizerFromNonExistentCluster(obj)
	if err != nil && !errors.IsNotFound(err) {
		return obj, err
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		obj.SetAnnotations(annotations)
	}
	obj.Annotations[grbstore.GrbVersion] = "true"
	return p.mgmt.Management.GlobalRoleBindings("").Update(obj)
}

func (p *grbCleaner) removeFinalizerFromNonExistentCluster(obj *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	obj = obj.DeepCopy()

	md, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	finalizers := md.GetFinalizers()
	for i := len(finalizers) - 1; i >= 0; i-- {
		f := finalizers[i]
		if strings.HasPrefix(f, lifecycle.ScopedFinalizerKey) {
			s := strings.Split(f, "_")
			// if cluster was not reported, its a deleted cluster and will cause the finalizer to hang in the future
			if _, err = p.clusterLister.Get("", s[1]); errors.IsNotFound(err) {
				finalizers = append(finalizers[:i], finalizers[i+1:]...)
			}
		}
	}

	md.SetFinalizers(finalizers)
	return obj, nil
}
