package auth

import (
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	roleTemplateHandlerName = "mgmt-auth-role-template-handler"
	prtbByRoleTemplateIndex = "management.cattle.io/prtb-by-role-template"
)

type roleTemplateHandler struct {
	prtbIndexer cache.Indexer
	prtbClient  v3.ProjectRoleTemplateBindingInterface
}

func newRoleTemplateHandler(management *config.ManagementContext) *roleTemplateHandler {
	informer := management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		prtbByRoleTemplateIndex: prtbByRoleTemplate,
	}
	informer.AddIndexers(indexers)

	rtl := &roleTemplateHandler{
		prtbIndexer: informer.GetIndexer(),
		prtbClient:  management.Management.ProjectRoleTemplateBindings(""),
	}
	return rtl
}

// sync is called periodically and on real updates
func (rtl *roleTemplateHandler) sync(key string, obj *v3.RoleTemplate) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	if err := rtl.enqueuePrtbs(obj); err != nil {
		return nil, err
	}
	return nil, nil
}

// enqueue any prtb's linked to this roleTemplate in order to re-sync it via reconcileBindings
func (rtl *roleTemplateHandler) enqueuePrtbs(updatedRT *v3.RoleTemplate) error {
	prtbs, err := rtl.prtbIndexer.ByIndex(prtbByRoleTemplateIndex, updatedRT.Name)
	if err != nil {
		return err
	}
	for _, x := range prtbs {
		if prtb, ok := x.(*v3.ProjectRoleTemplateBinding); ok {
			rtl.prtbClient.Controller().Enqueue(prtb.Namespace, prtb.Name)
		}
	}
	return nil
}

func prtbByRoleTemplate(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{prtb.RoleTemplateName}, nil
}
