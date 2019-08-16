package rbac

import (
	"strings"
	"time"

	"github.com/rancher/norman/lifecycle"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type rtCleaner struct {
	m *manager
}

func newLegacyRTCleaner(m *manager) *rtCleaner {
	return &rtCleaner{m: m}
}

func (p *rtCleaner) sync(key string, obj *v3.RoleTemplate) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}
	if obj.DeletionTimestamp == nil || len(obj.Finalizers) == 0 {
		return nil, nil
	}
	if time.Since(obj.DeletionTimestamp.Time) < 1*time.Hour {
		return nil, nil
	}
	obj, err := p.removeStuckFinalizer(obj)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	return obj, nil

}

func (p *rtCleaner) removeStuckFinalizer(obj *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	obj = obj.DeepCopy()
	modified := false
	md, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	finalizers := md.GetFinalizers()
	for i := len(finalizers) - 1; i >= 0; i-- {
		f := finalizers[i]
		if strings.HasPrefix(f, lifecycle.ScopedFinalizerKey) {
			finalizers = append(finalizers[:i], finalizers[i+1:]...)
			modified = true
		}
	}

	if modified {
		md.SetFinalizers(finalizers)
		obj, e := p.m.workload.Management.Management.RoleTemplates("").Update(obj)
		return obj, e
	}
	return obj, nil
}
