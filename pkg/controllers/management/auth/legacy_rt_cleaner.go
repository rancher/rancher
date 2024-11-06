package auth

import (
	"github.com/rancher/rancher/pkg/api/norman/store/roletemplate"
	wv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type rtCleaner struct {
	roleTemplates v3.RoleTemplateController
	clusterLister v3.ClusterController
}

func newLegacyRTCleaner(mgmt *config.ManagementContext) *rtCleaner {
	return &rtCleaner{
		roleTemplates: mgmt.Wrangler.Mgmt.RoleTemplate(),
		clusterLister: mgmt.Wrangler.Mgmt.Cluster(),
	}
}

// sync cleans up all roleTemplates to drop cluster-scoped lifecycle handler finalizers
func (p *rtCleaner) sync(key string, obj *wv3.RoleTemplate) (runtime.Object, error) {
	if key == "" || obj == nil {
		return nil, nil
	}
	if obj.Annotations[roletemplate.RTVersion] == "true" {
		return obj, nil
	}

	cleanedObj := rtCleanUp(obj)
	return p.roleTemplates.Update(cleanedObj)
}

// rtCleanUp returns a clean RoleTemplate based on filters specified within the function
func rtCleanUp(obj *wv3.RoleTemplate) *wv3.RoleTemplate {
	obj.SetFinalizers(cleanFinalizers(obj.GetFinalizers(), "clusterscoped.controller.cattle.io/cluster-roletemplate-sync_"))
	cleanAnnotations := cleanAnnotations(obj.GetAnnotations(), "lifecycle.cattle.io/create.cluster-roletemplate-sync_")

	if cleanAnnotations == nil {
		cleanAnnotations = make(map[string]string)
	}
	delete(cleanAnnotations, roletemplate.OldRTVersion)
	cleanAnnotations[roletemplate.RTVersion] = "true"
	obj.SetAnnotations(cleanAnnotations)
	return obj
}
