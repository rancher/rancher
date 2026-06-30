package operations

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func init() {
	RegisterAdapter(rkev1.SchemeGroupVersion.WithKind("RKEControlPlane"), func(clients *wrangler.CAPIContext, unstructured *unstructured.Unstructured) (Adapter, error) {
		controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(unstructured.GetNamespace(), unstructured.GetName())
		if err != nil {
			return nil, err
		}
		return &CAPRAdapter{
			controlPlane: controlPlane,
			clients:      clients,
		}, nil
	})
	RegisterAdapter(capiv1beta2.GroupVersion.WithKind("Cluster"), func(clients *wrangler.CAPIContext, unstructured *unstructured.Unstructured) (Adapter, error) {
		cluster, err := clients.CAPI.Cluster().Cache().Get(unstructured.GetNamespace(), unstructured.GetName())
		if err != nil {
			return nil, err
		}
		if cluster.Spec.ControlPlaneRef.APIGroup == rkev1.SchemeGroupVersion.Group && cluster.Spec.ControlPlaneRef.Kind == "RKEControlPlane" {
			controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(unstructured.GetNamespace(), unstructured.GetName())
			if err != nil {
				return nil, err
			}
			return &CAPRAdapter{
				controlPlane: controlPlane,
				clients:      clients,
			}, nil
		}
		if cluster.Spec.ControlPlaneRef.APIGroup == rkev1.SchemeGroupVersion.Group {}
		return nil, nil
	})
}