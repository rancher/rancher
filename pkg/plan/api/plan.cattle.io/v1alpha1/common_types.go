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
	// The lifecycle labels identify the cluster and machine objects that own a plan secret
	// (or a downstream Node). Only Group + Kind + Name are stamped:
	//   - Version is omitted because a GroupKind uniquely identifies a resource; the API server
	//     serves whichever version it stores in when a caller uses a discovery-mapped client.
	//   - Namespace is omitted deliberately — the caller's own context namespace is authoritative
	//     for resolving the reference. Encoding the namespace in a label would let a plan-secret
	//     value point at a resource in a different namespace than the secret itself, which is a
	//     cross-tenant spoofing vector.

	// ClusterLifecycleGroupLabel is the label key for specifying the group of the cluster associated with a plan secret.
	ClusterLifecycleGroupLabel = "plan.cattle.io/cluster-group"
	// ClusterLifecycleKindLabel is the label key for specifying the kind of the cluster associated with a plan secret.
	ClusterLifecycleKindLabel = "plan.cattle.io/cluster-kind"
	// ClusterLifecycleNameLabel is the label key for specifying the name of the cluster associated with a plan secret.
	ClusterLifecycleNameLabel = "plan.cattle.io/cluster-name"

	// MachineLifecycleGroupLabel is the label key for specifying the group of the machine associated with a plan secret.
	MachineLifecycleGroupLabel = "plan.cattle.io/machine-group"
	// MachineLifecycleKindLabel is the label key for specifying the kind of the machine associated with a plan secret.
	MachineLifecycleKindLabel = "plan.cattle.io/machine-kind"
	// MachineLifecycleNameLabel is the label key for specifying the name of the machine associated with a plan secret.
	MachineLifecycleNameLabel = "plan.cattle.io/machine-name"
)

// ObjToMachineLifecycleLabels returns the three-key lifecycle-label map that identifies a machine
// object. Kind + Group are read from the object's TypeMeta; cache-fetched objects typically have
// empty TypeMeta and callers must repopulate it before calling this.
func ObjToMachineLifecycleLabels(obj runtime.Object) (map[string]string, error) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	return map[string]string{
		MachineLifecycleGroupLabel: gvk.Group,
		MachineLifecycleKindLabel:  gvk.Kind,
		MachineLifecycleNameLabel:  metaObj.GetName(),
	}, nil
}

// ObjToClusterLifecycleLabels returns the three-key lifecycle-label map that identifies a cluster
// object. See ObjToMachineLifecycleLabels for the TypeMeta caveat.
func ObjToClusterLifecycleLabels(obj runtime.Object) (map[string]string, error) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	return map[string]string{
		ClusterLifecycleGroupLabel: gvk.Group,
		ClusterLifecycleKindLabel:  gvk.Kind,
		ClusterLifecycleNameLabel:  metaObj.GetName(),
	}, nil
}

// HasMachineLifecycleLabels reports whether obj carries a complete machine-lifecycle label triple.
func HasMachineLifecycleLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	return labels[MachineLifecycleGroupLabel] != "" &&
		labels[MachineLifecycleKindLabel] != "" &&
		labels[MachineLifecycleNameLabel] != ""
}

// HasClusterLifecycleLabels reports whether obj carries a complete cluster-lifecycle label triple.
func HasClusterLifecycleLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	return labels[ClusterLifecycleGroupLabel] != "" &&
		labels[ClusterLifecycleKindLabel] != "" &&
		labels[ClusterLifecycleNameLabel] != ""
}

// ResolveKindStorageVersion asks the discovery-backed RESTMapper for the storage-served
// (group, version, kind) triple and the scope of the given GroupKind. Callers use this to turn
// the label-carried (Group, Kind) into the full GVK that dynamic clients require.
func ResolveKindStorageVersion(mapper meta.RESTMapper, gk schema.GroupKind) (schema.GroupVersionKind, meta.RESTScope, error) {
	mapping, err := mapper.RESTMapping(gk)
	if err != nil {
		return schema.GroupVersionKind{}, nil, err
	}
	return mapping.GroupVersionKind, mapping.Scope, nil
}

// MachineLifecycleLabelsToObjectReference parses the machine-lifecycle labels on obj into an
// ObjectReference. The Namespace field of the returned reference is ALWAYS contextNamespace —
// the caller supplies its own authoritative namespace; this is what prevents cross-namespace
// spoofing through label values. The APIVersion is resolved from the labelled Group via the
// RESTMapper.
func MachineLifecycleLabelsToObjectReference(obj metav1.Object, contextNamespace string, mapper meta.RESTMapper) (*corev1.ObjectReference, error) {
	return lifecycleLabelsToObjectReference(obj, contextNamespace, mapper,
		MachineLifecycleGroupLabel, MachineLifecycleKindLabel, MachineLifecycleNameLabel, "machine")
}

// ClusterLifecycleLabelsToObjectReference is the cluster-lifecycle analogue of
// MachineLifecycleLabelsToObjectReference. When the resolved scope is Root (cluster-scoped —
// e.g. management.cattle.io/v3 Cluster) the returned reference has Namespace = "" regardless of
// contextNamespace.
func ClusterLifecycleLabelsToObjectReference(obj metav1.Object, contextNamespace string, mapper meta.RESTMapper) (*corev1.ObjectReference, error) {
	return lifecycleLabelsToObjectReference(obj, contextNamespace, mapper,
		ClusterLifecycleGroupLabel, ClusterLifecycleKindLabel, ClusterLifecycleNameLabel, "cluster")
}

func lifecycleLabelsToObjectReference(obj metav1.Object, contextNamespace string, mapper meta.RESTMapper, groupKey, kindKey, nameKey, side string) (*corev1.ObjectReference, error) {
	prefix := fmt.Sprintf("object %s", obj.GetName())
	if obj.GetNamespace() != "" {
		prefix = fmt.Sprintf("object %s/%s", obj.GetNamespace(), obj.GetName())
	}

	labels := obj.GetLabels()
	if labels == nil {
		return nil, fmt.Errorf("%s has no labels", prefix)
	}

	group, kind, name := labels[groupKey], labels[kindKey], labels[nameKey]
	if kind == "" {
		return nil, fmt.Errorf("%s has no %s kind label", prefix, side)
	}
	if name == "" {
		return nil, fmt.Errorf("%s has no %s name label", prefix, side)
	}

	gvk, scope, err := ResolveKindStorageVersion(mapper, schema.GroupKind{Group: group, Kind: kind})
	if err != nil {
		return nil, fmt.Errorf("%s: resolving %s/%s: %w", prefix, group, kind, err)
	}

	ns := contextNamespace
	if scope != nil && scope.Name() == meta.RESTScopeNameRoot {
		ns = ""
	}
	return &corev1.ObjectReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       name,
		Namespace:  ns,
	}, nil
}
