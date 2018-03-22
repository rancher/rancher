package podsecuritypolicy

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const psptpbByPSPTNameIndex = "something.something.psptpb/pspt-name"

func Register(management *config.ManagementContext) {
	psptpbInformer := management.Management.PodSecurityPolicyTemplateProjectBindings("").Controller().Informer()
	psptpbIndexers := map[string]cache.IndexFunc{
		psptpbByPSPTNameIndex: PSPTPBByPSPTName,
	}
	psptpbInformer.AddIndexers(psptpbIndexers)

	lifecycle := &lifecycle{
		psptpbs:      management.Management.PodSecurityPolicyTemplateProjectBindings(""),
		psptpbLister: management.Management.PodSecurityPolicyTemplateProjectBindings("").Controller().Lister(),

		psptpbIndexer: psptpbInformer.GetIndexer(),
	}

	management.Management.PodSecurityPolicyTemplates("").AddLifecycle("mgmt-pspt-lfc-handler", lifecycle)
}

type lifecycle struct {
	psptpbs      v3.PodSecurityPolicyTemplateProjectBindingInterface
	psptpbLister v3.PodSecurityPolicyTemplateProjectBindingLister

	psptpbIndexer cache.Indexer
}

func (l *lifecycle) Create(obj *v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error) {
	return obj, nil
}

func (l *lifecycle) Updated(obj *v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error) {
	return obj, nil
}

func (l *lifecycle) Remove(obj *v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error) {
	psptpbs, err := l.psptpbIndexer.ByIndex(psptpbByPSPTNameIndex, obj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting psptpbs: %v", err)
	}

	for _, rawPSPTPB := range psptpbs {
		psptpb := rawPSPTPB.(*v3.PodSecurityPolicyTemplateProjectBinding)
		err := l.psptpbs.Delete(psptpb.Name, &v1.DeleteOptions{})
		if err != nil {
			return nil, fmt.Errorf("error deleting psptpb: %v", err)
		}
	}

	return obj, nil
}

func PSPTPBByPSPTName(obj interface{}) ([]string, error) {
	psptpb, ok := obj.(*v3.PodSecurityPolicyTemplateProjectBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{psptpb.PodSecurityPolicyTemplateName}, nil
}
