package auth

import (
	"strings"

	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/rancher/pkg/api/store/roletemplate"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type rtCleaner struct {
	clusterLister v3.ClusterLister
	mgmt          *config.ManagementContext
}

func newLegacyRTCleaner(mgmt *config.ManagementContext) *rtCleaner {
	return &rtCleaner{
		mgmt:          mgmt,
		clusterLister: mgmt.Management.Clusters("").Controller().Lister(),
	}

}

func (p *rtCleaner) sync(key string, obj *v3.RoleTemplate) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}
	if obj.Annotations[roletemplate.RTVersion] == "true" {
		return obj, nil
	}

	obj, err := p.removeFinalizerFromNonExistentCluster(obj)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		obj.SetAnnotations(annotations)
	}
	obj.Annotations[roletemplate.RTVersion] = "true"
	return p.mgmt.Management.RoleTemplates("").Update(obj)
}

func (p *rtCleaner) removeFinalizerFromNonExistentCluster(obj *v3.RoleTemplate) (*v3.RoleTemplate, error) {
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
