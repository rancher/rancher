package v1alpha1

import (
	"fmt"
	"strings"

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

// Phase hook label prefixes are the shared "<phase>.phase.hook.operation.cattle.io/" namespace used
// to gate operation progression at phase boundaries. They are common to every operation type
// (ETCDSnapshotSave, ETCDSnapshotRestore, EncryptionKeyRotation, …) because every operation goes
// through the same phase state machine (Pending → InProgress → Succeeded | Failed | Canceled).
// Step-level hooks are operation-specific and live alongside their controller.
//
// Semantics: when an Operation carries a label whose key starts with one of these prefixes, the
// owning controller pushes the label's value onto the cluster beacon's delegate chain on entry
// to the matching phase and then short-circuits further work in that phase. The operation
// resumes only after BOTH:
//
//   - the label is removed from the Operation (otherwise the next reconcile re-pushes the same
//     delegate onto the chain), and
//   - the delegate is popped from the beacon's delegate chain (otherwise the controller still
//     sees the delegate as the current beacon authority).
//
// The label-key suffix after the prefix is the hook's identifier (e.g.
// "<prefix>/my-cleanup-hook"); the controller does not interpret it. The label VALUE is the name
// of the delegate pushed onto the beacon chain — a cooperating controller subscribes to that
// delegate name to know when its turn arrives, and pops itself off the chain when finished.
//
// Use these prefixes when adding cross-cutting hooks that should run at the same phase point
// across every operation type. For operation-type-specific gating (e.g. between snapshot Save and
// Restart, or between EKR Rotate and Restart) use the step-level prefixes exported by the
// respective controller package.
const (
	// PendingPhaseHookLabelPrefix gates the Pending phase, after the controller has acquired the
	// cluster beacon but before it waits for system-agents to register. A delegate hooked here
	// observes the cluster in its pre-operation state.
	PendingPhaseHookLabelPrefix = "pending.phase.hook.operation.cattle.io/"

	// InProgressPhaseHookLabelPrefix gates every InProgress reconcile, ahead of step dispatch.
	// Useful for delegates that need to gate ALL step work uniformly without subscribing to each
	// step prefix individually.
	InProgressPhaseHookLabelPrefix = "in-progress.phase.hook.operation.cattle.io/"

	// CanceledPhaseHookLabelPrefix gates the Canceled phase, before the controller releases the
	// beacon and runs any operation-type-specific cleanup (e.g. unpausing the CAPI cluster on
	// encryption-key-rotation). Lets a delegate inspect / react to the cancellation cause.
	CanceledPhaseHookLabelPrefix = "canceled.phase.hook.operation.cattle.io/"

	// FailedPhaseHookLabelPrefix gates the Failed phase, before the controller releases the
	// beacon and runs any operation-type-specific cleanup. Lets a delegate inspect failure state
	// (status conditions, plan-secret applied output, residual node-side scripts) before the next
	// operation is allowed to acquire the beacon.
	FailedPhaseHookLabelPrefix = "failed.phase.hook.operation.cattle.io/"

	// SucceededPhaseHookLabelPrefix gates the Succeeded phase, before the controller releases the
	// beacon. Lets a delegate chain follow-up work (e.g. snapshotbackpopulate after a restore, an
	// external verifier after a key rotation) before the cluster accepts new operations.
	SucceededPhaseHookLabelPrefix = "succeeded.phase.hook.operation.cattle.io/"
)

// hookLabelKeyMarker is the substring shared by every phase-hook and step-hook label key. Detecting
// it is enough to know "some delegate has posted a hook here" without enumerating every registered
// prefix — an operation-type-specific step prefix registered in a controller package still matches.
const hookLabelKeyMarker = ".hook.operation.cattle.io/"

// HasActiveLifecycleHook reports whether obj still carries at least one lifecycle-hook label
// (phase or step). While such a label is present, the op's owning controller MUST NOT garbage
// collect the object even after its terminal phase and TTL have expired — the delegate needs a
// chance to observe the current phase and pop itself from the beacon's delegate chain, and it
// signals it is done by removing the label.
//
// Recognises any label key that contains the shared hook-namespace marker
// ".hook.operation.cattle.io/". This catches all phase prefixes defined above and every
// step-level prefix exported by an operation controller package, so callers do not have to enumerate.
func HasActiveLifecycleHook(obj metav1.Object) bool {
	if obj == nil {
		return false
	}
	for k := range obj.GetLabels() {
		if strings.Contains(k, hookLabelKeyMarker) {
			return true
		}
	}
	return false
}

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
