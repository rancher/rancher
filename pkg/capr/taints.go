package capr

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// DefaultTaints are the taints automatically managed by Rancher for RKE2/K3s clusters.
// These taints are added/removed based on machine roles.
var DefaultTaints = map[string]corev1.Taint{
	"control-plane": {
		Key:    "node-role.kubernetes.io/control-plane",
		Effect: corev1.TaintEffectNoSchedule,
	},
	"etcd": {
		Key:    "node-role.kubernetes.io/etcd",
		Effect: corev1.TaintEffectNoExecute,
	},
}

// GetExpectedDefaultTaints calculates the expected default taints for a machine
// based on its role labels and the cluster runtime (K3s/RKE2).
//
// Rules:
// - If machine has worker role: no default taints
// - If machine has control plane role: add control-plane taint
// - If machine has etcd role: add etcd taint (except for K3s when also control-plane)
func GetExpectedDefaultTaints(machine *capi.Machine, runtime string) []corev1.Taint {
	if machine == nil || machine.Labels == nil {
		return nil
	}

	var result []corev1.Taint

	hasWorker := machine.Labels[WorkerRoleLabel] == "true"
	hasControlPlane := machine.Labels[ControlPlaneRoleLabel] == "true"
	hasEtcd := machine.Labels[EtcdRoleLabel] == "true"

	// If node has worker role, it should not have default taints
	if hasWorker {
		return result
	}

	// Add etcd taint if:
	// - Node has etcd role AND
	// - (Node doesn't have control plane role OR runtime is not K3s)
	//
	// K3s special case: when a node is both control-plane and etcd,
	// don't add the etcd taint because K3s charts don't have correct
	// tolerations for this scenario.
	if hasEtcd && (!hasControlPlane || runtime != RuntimeK3S) {
		result = append(result, DefaultTaints["etcd"])
	}

	// Add control-plane taint if node has control plane role
	if hasControlPlane {
		result = append(result, DefaultTaints["control-plane"])
	}

	return result
}

// IsDefaultTaint checks if a taint is one of the default managed taints.
func IsDefaultTaint(taint corev1.Taint) bool {
	for _, dt := range DefaultTaints {
		if taint.Key == dt.Key && taint.Effect == dt.Effect {
			return true
		}
	}
	return false
}

// ParseTaintsAnnotation parses the taints annotation from a machine.
// Returns nil if the annotation is not present or empty.
func ParseTaintsAnnotation(annotations map[string]string) ([]corev1.Taint, error) {
	if annotations == nil {
		return nil, nil
	}

	data := annotations[TaintsAnnotation]
	if data == "" {
		return nil, nil
	}

	var taints []corev1.Taint
	if err := json.Unmarshal([]byte(data), &taints); err != nil {
		return nil, err
	}
	return taints, nil
}
