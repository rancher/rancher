package authprovisioningv2

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/relatedresource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// enqueueGRBAfterCRTBChange re-enqueues an individual GRB that owns a CRTB that got changed.
// When a CRTB for a restricted admin gets deleted, this method will re-enqueue the GRB, and that will trigger the
// re-creation of the CRTB.
func (h *handler) enqueueGRBAfterCRTBChange(namespace, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok {
		return nil, nil
	}

	ref := findGRBOwnerReferenceInCRTB(crtb)
	if ref == nil {
		return nil, nil
	}

	grb, err := h.globalRoleBindingsLister.Get(ref.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return []relatedresource.Key{
		{
			Name: grb.Name,
		},
	}, nil
}

func findGRBOwnerReferenceInCRTB(b *v3.ClusterRoleTemplateBinding) *v1.OwnerReference {
	if b == nil || len(b.OwnerReferences) == 0 {
		return nil
	}
	for _, ref := range b.OwnerReferences {
		if ref.Kind == "GlobalRoleBinding" {
			return &ref
		}
	}
	return nil
}
