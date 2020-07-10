package podsecuritypolicy

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	v1beta12 "github.com/rancher/rancher/pkg/types/apis/policy/v1beta1"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/api/policy/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func RegisterPodSecurityPolicy(ctx context.Context, context *config.UserContext) {
	p := pspHandler{
		psptLister:          context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		podSecurityPolicies: context.Policy.PodSecurityPolicies(""),
	}

	context.Policy.PodSecurityPolicies("").AddHandler(ctx, "psp-sync", p.sync)
}

type pspHandler struct {
	psptLister          v3.PodSecurityPolicyTemplateLister
	podSecurityPolicies v1beta12.PodSecurityPolicyInterface
}

// sync checks if a psp has a parent pspt based on the annotation and if that parent no longer
// exists will delete the psp
func (p *pspHandler) sync(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	if templateID, ok := obj.Annotations[podSecurityPolicyTemplateParentAnnotation]; ok {
		_, err := p.psptLister.Get("", templateID)
		if err != nil {
			// parent template is gone, delete the psp
			if k8serrors.IsNotFound(err) {
				return obj, p.podSecurityPolicies.Delete(obj.Name, &metav1.DeleteOptions{})

			}
			return obj, err
		}

	}
	return obj, nil
}
