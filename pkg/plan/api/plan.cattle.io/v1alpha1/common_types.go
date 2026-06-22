package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	//
	BeaconOwnerLabel    = "plan.cattle.io/owner"

	//
	BeaconDelegateLabel = "plan.cattle.io/delegate"

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

func HasMachineLifecycleLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	group := labels[MachineLifecycleGroup]
	if group == "" {
		return false
	}
	version := labels[MachineLifecycleVersion]
	if version == "" {
		return false
	}
	kind := labels[MachineLifecycleKind]
	if kind == "" {
		return false
	}
	// theoretically could be a non-namespaced resource, but in practice this doesn't exist
	namespace := labels[MachineLifecycleNamespace]
	if namespace == "" {
		return false
	}
	name := labels[MachineLifecycleName]
	return name != ""
}

func MachineLifecycleLabelsToObjectReference(obj metav1.Object) (*corev1.ObjectReference, error) {
	prefix := fmt.Sprintf("object %s", obj.GetName())
	if obj.GetNamespace() != "" {
		prefix = fmt.Sprintf("object %s/%s", obj.GetNamespace(), obj.GetName())
	}

	labels := obj.GetLabels()
	if labels == nil {
		return nil, fmt.Errorf("%s has no labels", prefix)
	}

	group := labels[MachineLifecycleGroup]
	if group == "" {
		return nil, fmt.Errorf("%s has no group label", prefix)
	}

	version := labels[MachineLifecycleVersion]
	if version == "" {
		return nil, fmt.Errorf("%s has no version label", prefix)
	}

	kind := labels[MachineLifecycleKind]
	if kind == "" {
		return nil, fmt.Errorf("%s has no kind label", prefix)
	}

	namespace := labels[MachineLifecycleNamespace]
	if namespace == "" {
		return nil, fmt.Errorf("%s has no namespace label", prefix)
	}

	name := labels[MachineLifecycleName]
	if name == "" {
		return nil, fmt.Errorf("%s has no name label", prefix)
	}

	gvr := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
	return &corev1.ObjectReference{
		APIVersion: gvr.GroupVersion().String(),
		Kind:       gvr.Kind,
		Name:       name,
		Namespace:  namespace,
	}, nil
}
