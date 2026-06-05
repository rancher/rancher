package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

const BeaconOwnerLabel = "plan.cattle.io/owner"

const (
	ClusterLifecycleGroup     = "plan.cattle.io/cluster-group"
	ClusterLifecycleVersion   = "plan.cattle.io/cluster-version"
	ClusterLifecycleKind      = "plan.cattle.io/cluster-kind"
	ClusterLifecycleNamespace = "plan.cattle.io/cluster-namespace"
	ClusterLifecycleName      = "plan.cattle.io/cluster-name"

	MachineLifecycleGroup     = "plan.cattle.io/machine-group"
	MachineLifecycleVersion   = "plan.cattle.io/machine-version"
	MachineLifecycleKind      = "plan.cattle.io/machine-kind"
	MachineLifecycleNamespace = "plan.cattle.io/machine-namespace"
	MachineLifecycleName      = "plan.cattle.io/machine-name"
)

func ObjToMachineLifecycleLabels(obj runtime.Object) (map[string]string, error) {
	labels := make(map[string]string, 5)

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	labels[MachineLifecycleGroup] = gvk.Group

	return map[string]string{
		MachineLifecycleGroup:     gvk.Group,
		MachineLifecycleVersion:   gvk.Version,
		MachineLifecycleKind:      gvk.Kind,
		MachineLifecycleName:      metaObj.GetName(),
		MachineLifecycleNamespace: metaObj.GetNamespace(),
	}, nil
}

func ObjToClusterLifecycleLabels(obj runtime.Object) (map[string]string, error) {
	labels := make(map[string]string, 5)

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	labels[ClusterLifecycleGroup] = gvk.Group

	return map[string]string{
		ClusterLifecycleGroup:     gvk.Group,
		ClusterLifecycleVersion:   gvk.Version,
		ClusterLifecycleKind:      gvk.Kind,
		ClusterLifecycleName:      metaObj.GetName(),
		ClusterLifecycleNamespace: metaObj.GetNamespace(),
	}, nil
}
