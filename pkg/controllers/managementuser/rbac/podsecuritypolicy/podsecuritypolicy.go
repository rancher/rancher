package podsecuritypolicy

import (
	"context"
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1beta12 "github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
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
		clusterLister:       context.Management.Management.Clusters("").Controller().Lister(),
		clusterName:         context.ClusterName,
	}

	context.Policy.PodSecurityPolicies("").AddHandler(ctx, "psp-sync", p.sync)
}

type pspHandler struct {
	psptLister          v3.PodSecurityPolicyTemplateLister
	podSecurityPolicies v1beta12.PodSecurityPolicyInterface
	clusterLister       v3.ClusterLister
	clusterName         string
}

// sync checks if a psp has a parent pspt based on the annotation and if that parent no longer
// exists will delete the psp
func (p *pspHandler) sync(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	err := CheckClusterVersion(p.clusterName, p.clusterLister)
	if err != nil {
		if errors.Is(err, ErrClusterVersionIncompatible) {
			return obj, nil
		}
		return obj, fmt.Errorf("error checking cluster version for PodSecurityPolicy controller: %w", err)
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
