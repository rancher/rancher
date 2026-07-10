package operations

import (
	"fmt"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

var (
	errUnsupportedClusterType = fmt.Errorf("unsupported cluster type")
)

func init() {
	RegisterAdapter(capiv1beta2.GroupVersion.WithKind("Cluster"), func(clients *wrangler.CAPIContext, ustr *unstructured.Unstructured) (Adapter, error) {
		cluster, err := clients.CAPI.Cluster().Cache().Get(ustr.GetNamespace(), ustr.GetName())
		if err != nil {
			return nil, err
		}

		return capiClusterAdapter(clients, cluster, "")
	})
}

// capiClusterAdapter returns the appropriate Adapter for a CAPI Cluster based on its
// controlPlaneRef: RKEControlPlane → CAPRAdapter, RKE2ControlPlane → CAPRKE2Adapter. Returns
// (nil, nil) when the control-plane ref points at an unsupported kind (caller should treat this
// as unsupported cluster type). Callers that reach this via the mgmt-cluster dispatch path (see
// imported.go) rely on this helper to keep the CAPI-side logic in one place.
//
// mgmtClusterName is the mgmt v3 Cluster shell name for turtles-imported dispatch paths (empty
// when called from the direct-CAPI factory). It is threaded into CAPRKE2Adapter so it can
// resolve the mgmt-side snapshot namespace independently of the CAPI Cluster's namespace.
func capiClusterAdapter(clients *wrangler.CAPIContext, cluster *capiv1beta2.Cluster, mgmtClusterName string) (Adapter, error) {
	if cluster.Spec.ControlPlaneRef.APIGroup == controlplanev1beta2.GroupVersion.Group && cluster.Spec.ControlPlaneRef.Kind == "RKE2ControlPlane" {
		obj, err := clients.Dynamic.Get(controlplanev1beta2.GroupVersion.WithKind("RKE2ControlPlane"), cluster.Namespace, cluster.Name)
		if err != nil {
			return nil, err
		}

		// clients.Dynamic.Get returns an untyped runtime.Object. Convert it through the unstructured
		// form into the typed *caprke2v1beta2.RKE2ControlPlane so the adapter can read spec fields
		// directly (mirrors how CAPRAdapter holds a typed *rkev1.RKEControlPlane).
		cpUstr, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("expected *unstructured.Unstructured for RKE2ControlPlane %s/%s, got %T", cluster.Namespace, cluster.Name, obj)
		}

		controlPlane := &controlplanev1beta2.RKE2ControlPlane{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(cpUstr.Object, controlPlane); err != nil {
			return nil, fmt.Errorf("converting RKE2ControlPlane %s/%s from unstructured: %w", cluster.Namespace, cluster.Name, err)
		}

		return &CAPRKE2Adapter{
			cluster:         cluster,
			controlPlane:    controlPlane,
			clients:         clients,
			mgmtClusterName: mgmtClusterName,
		}, nil
	}

	return nil, errUnsupportedClusterType
}
