package persistentvolumes

import (
	corev1 "k8s.io/api/core/v1"
)

// NewNodeSelectorRequirement is a constructor for a NodeSelectorRequirement config object for a Persistent Volume
// in downstream cluster. `operator` is a const from the corev1 "k8s.io/api/core/v1" package
func NewNodeSelectorRequirement(operator corev1.NodeSelectorOperator, key string, values ...string) corev1.NodeSelectorRequirement {
	return corev1.NodeSelectorRequirement{
		Key:      key,
		Operator: operator,
		Values:   values,
	}
}
