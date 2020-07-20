package auth

import (
	"strings"

	grbstore "github.com/rancher/rancher/pkg/api/norman/store/globalrolebindings"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
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

	obj.SetFinalizers(cleanFinalizers(obj.GetFinalizers(), "clusterscoped.controller.cattle.io/grb-sync_"))
	cleanAnnotations := cleanAnnotations(obj.GetAnnotations(), "lifecycle.cattle.io/create.grb-sync_")

	if cleanAnnotations == nil {
		cleanAnnotations = make(map[string]string)
	}
	delete(cleanAnnotations, grbstore.OldGrbVersion)
	cleanAnnotations[grbstore.GrbVersion] = "true"
	obj.SetAnnotations(cleanAnnotations)
	return p.mgmt.Management.GlobalRoleBindings("").Update(obj)
}

// cleanFinalizers takes a list of finalizers and removes any finalizer that has the matching prefix
func cleanFinalizers(finalizers []string, prefix string) []string {
	var newFinalizers []string
	for _, finalizer := range finalizers {
		if strings.HasPrefix(finalizer, prefix) {
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}
	return newFinalizers
}

// cleanAnnotations takes an objects annotations and removes any annotation that has the matching prefix
// returning a new map
func cleanAnnotations(annotations map[string]string, prefix string) map[string]string {
	newAnnos := make(map[string]string)
	for k, v := range annotations {
		if strings.HasPrefix(k, prefix) {
			continue
		}
		newAnnos[k] = v
	}
	return newAnnos
}
