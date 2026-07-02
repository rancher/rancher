package operations

import (
	"fmt"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func init() {
	RegisterAdapter(rkev1.SchemeGroupVersion.WithKind("RKEControlPlane"), func(clients *wrangler.CAPIContext, ustr *unstructured.Unstructured) (Adapter, error) {
		controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(ustr.GetNamespace(), ustr.GetName())
		if err != nil {
			return nil, err
		}
		return &CAPRAdapter{
			controlPlane: controlPlane,
			clients:      clients,
		}, nil
	})
	RegisterAdapter(capiv1beta2.GroupVersion.WithKind("Cluster"), func(clients *wrangler.CAPIContext, ustr *unstructured.Unstructured) (Adapter, error) {
		cluster, err := clients.CAPI.Cluster().Cache().Get(ustr.GetNamespace(), ustr.GetName())
		if err != nil {
			return nil, err
		}
		if cluster.Spec.ControlPlaneRef.APIGroup == rkev1.SchemeGroupVersion.Group && cluster.Spec.ControlPlaneRef.Kind == "RKEControlPlane" {
			controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(ustr.GetNamespace(), ustr.GetName())
			if err != nil {
				return nil, err
			}
			return &CAPRAdapter{
				controlPlane: controlPlane,
				clients:      clients,
			}, nil
		}
		if cluster.Spec.ControlPlaneRef.APIGroup == controlplanev1beta2.GroupVersion.Group && cluster.Spec.ControlPlaneRef.Kind == "RKE2ControlPlane" {
			obj, err := clients.Dynamic.Get(controlplanev1beta2.GroupVersion.WithKind("RKE2ControlPlane"), ustr.GetNamespace(), ustr.GetName())
			if err != nil {
				return nil, err
			}
			// clients.Dynamic.Get returns an untyped runtime.Object. Convert it through the unstructured
			// form into the typed *caprke2v1beta2.RKE2ControlPlane so the adapter can read spec fields
			// directly (mirrors how CAPRAdapter holds a typed *rkev1.RKEControlPlane).
			cpUstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return nil, fmt.Errorf("expected *unstructured.Unstructured for RKE2ControlPlane %s/%s, got %T", ustr.GetNamespace(), ustr.GetName(), obj)
			}
			controlPlane := &controlplanev1beta2.RKE2ControlPlane{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(cpUstr.Object, controlPlane); err != nil {
				return nil, fmt.Errorf("converting RKE2ControlPlane %s/%s from unstructured: %w", ustr.GetNamespace(), ustr.GetName(), err)
			}
			return &CAPRKE2Adapter{
				controlPlane: controlPlane,
				clients:      clients,
			}, nil
		}
		return nil, nil
	})
}
