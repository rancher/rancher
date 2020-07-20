package auth

import (
	"github.com/rancher/rancher/pkg/api/norman/store/roletemplate"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
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

// sync cleans up all roleTemplates to drop cluster-scoped lifecycle handler finalizers
func (p *rtCleaner) sync(key string, obj *v3.RoleTemplate) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}
	if obj.Annotations[roletemplate.RTVersion] == "true" {
		return obj, nil
	}

	obj.SetFinalizers(cleanFinalizers(obj.GetFinalizers(), "clusterscoped.controller.cattle.io/cluster-roletemplate-sync_"))
	cleanAnnotations := cleanAnnotations(obj.GetAnnotations(), "lifecycle.cattle.io/create.cluster-roletemplate-sync_")

	if cleanAnnotations == nil {
		cleanAnnotations = make(map[string]string)
	}
	delete(cleanAnnotations, roletemplate.OldRTVersion)
	cleanAnnotations[roletemplate.RTVersion] = "true"
	obj.SetAnnotations(cleanAnnotations)
	return p.mgmt.Management.RoleTemplates("").Update(obj)
}
