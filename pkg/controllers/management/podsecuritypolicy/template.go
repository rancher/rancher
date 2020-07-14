package podsecuritypolicy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const psptpbByPSPTNameIndex = "something.something.psptpb/pspt-name"

func Register(ctx context.Context, management *config.ManagementContext) {
	psptpbInformer := management.Management.PodSecurityPolicyTemplateProjectBindings("").Controller().Informer()
	psptpbIndexers := map[string]cache.IndexFunc{
		psptpbByPSPTNameIndex: PSPTPBByPSPTName,
	}
	psptpbInformer.AddIndexers(psptpbIndexers)

	lifecycle := &lifecycle{
		psptpbs:       management.Management.PodSecurityPolicyTemplateProjectBindings(""),
		psptpbIndexer: psptpbInformer.GetIndexer(),
	}

	management.Management.PodSecurityPolicyTemplates("").AddLifecycle(ctx, "mgmt-pspt-lfc-handler", lifecycle)
}

type lifecycle struct {
	psptpbs       v3.PodSecurityPolicyTemplateProjectBindingInterface
	psptpbIndexer cache.Indexer
}

func (l *lifecycle) Create(obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error) {
	return obj, nil
}

func (l *lifecycle) Updated(obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error) {
	return obj, nil
}

func (l *lifecycle) Remove(obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error) {
	psptpbs, err := l.psptpbIndexer.ByIndex(psptpbByPSPTNameIndex, obj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting psptpbs: %v", err)
	}

	for _, rawPSPTPB := range psptpbs {
		psptpb := rawPSPTPB.(*v3.PodSecurityPolicyTemplateProjectBinding)
		err := l.psptpbs.DeleteNamespaced(psptpb.Namespace, psptpb.Name, &v1.DeleteOptions{})
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
