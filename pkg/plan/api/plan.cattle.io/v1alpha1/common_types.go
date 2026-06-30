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
	BeaconOwnerLabel = "plan.cattle.io/owner"

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
