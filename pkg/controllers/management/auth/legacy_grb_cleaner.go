package auth

import (
	"maps"
	"slices"
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

	cleanedObj := gbrCleanUp(obj)
	return p.mgmt.Management.GlobalRoleBindings("").Update(cleanedObj)
}

// gbrCleanUp returns a clean GlobalRoleBinding based on filters specified within the function
func gbrCleanUp(obj *v3.GlobalRoleBinding) *v3.GlobalRoleBinding {
	// set finalizers to the object by filtering out
	// the ones that start with "clusterscoped.controller.cattle.io/grb-sync_"
	// and then clean annotations that start with "lifecycle.cattle.io/create.grb-sync_"
	obj.SetFinalizers(cleanFinalizers(obj.GetFinalizers(), "clusterscoped.controller.cattle.io/grb-sync_"))
	cleanAnnotations := cleanAnnotations(obj.GetAnnotations(), "lifecycle.cattle.io/create.grb-sync_")

	if cleanAnnotations == nil {
		cleanAnnotations = make(map[string]string)
	}
	delete(cleanAnnotations, grbstore.OldGrbVersion)
	cleanAnnotations[grbstore.GrbVersion] = "true"
	obj.SetAnnotations(cleanAnnotations)
	return obj
}

// cleanFinalizers takes a list of finalizers and removes any finalizer that has the matching prefix
func cleanFinalizers(finalizers []string, prefix string) []string {
	filteredFinalizers := slices.DeleteFunc(finalizers, func(s string) bool {
		return strings.HasPrefix(s, prefix)
	})
	return filteredFinalizers
}

// cleanAnnotations takes an objects annotations and removes any annotation that has the matching prefix
// returning a new map
func cleanAnnotations(annotations map[string]string, prefix string) map[string]string {
	filteredAnnotations := maps.Clone(annotations)
	maps.DeleteFunc(filteredAnnotations, func(key string, value string) bool {
		return strings.HasPrefix(key, prefix)
	})
	return filteredAnnotations
}
