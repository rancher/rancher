package auth

import (
	"strings"

	grbstore "github.com/rancher/rancher/pkg/api/store/globalrolebindings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type grbCleaner struct {
	mgmt *config.ManagementContext
}

func newLegacyGRBCleaner(m *config.ManagementContext) *grbCleaner {
	return &grbCleaner{
		mgmt: m,
	}
}

// sync cleans up all GRBs to drop cluster-scoped lifecycle handler finalizers
func (p *grbCleaner) sync(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}
	if obj.Annotations[grbstore.GrbVersion] == "true" {
		return obj, nil
	}

	obj = p.cleanFinalizers(obj)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		obj.SetAnnotations(annotations)
	}
	obj.Annotations[grbstore.GrbVersion] = "true"
	return p.mgmt.Management.GlobalRoleBindings("").Update(obj)
}

func (p *grbCleaner) cleanFinalizers(obj *v3.GlobalRoleBinding) *v3.GlobalRoleBinding {
	var newFinalizers []string
	for _, finalizer := range obj.GetFinalizers() {
		if strings.HasPrefix(finalizer, "clusterscoped.controller.cattle.io/grb-sync_") {
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}
	obj.SetFinalizers(newFinalizers)
	return obj
}
