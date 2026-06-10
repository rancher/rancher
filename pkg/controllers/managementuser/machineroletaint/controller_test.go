package machineroletaint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestTaintsNeedUpdate(t *testing.T) {
	h := &handler{}

	tests := []struct {
		name                 string
		nodeTaints           []corev1.Taint
		expectedTaints       []corev1.Taint
		expectUpdate         bool
		expectedToAddCount   int
		expectedToRemoveCount int
	}{
		{
			name:                 "no taints - no update needed",
			nodeTaints:           nil,
			expectedTaints:       nil,
			expectUpdate:         false,
			expectedToAddCount:   0,
			expectedToRemoveCount: 0,
		},
		{
			name: "add control-plane taint",
			nodeTaints:           nil,
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:         true,
			expectedToAddCount:   1,
			expectedToRemoveCount: 0,
		},
		{
			name: "remove control-plane taint",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints:       nil,
			expectUpdate:         true,
			expectedToAddCount:   0,
			expectedToRemoveCount: 1,
		},
		{
			name: "preserve user taints",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				{Key: "custom-taint", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints:       nil,
			expectUpdate:         true,
			expectedToAddCount:   0,
			expectedToRemoveCount: 1, // Only remove control-plane, preserve custom
		},
		{
			name: "taints match - no update",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:         false,
			expectedToAddCount:   0,
			expectedToRemoveCount: 0,
		},
		{
			name: "swap taints - remove etcd, add control-plane",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/etcd", Effect: corev1.TaintEffectNoExecute},
			},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:         true,
			expectedToAddCount:   1,
			expectedToRemoveCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toAdd, toRemove, needsUpdate := h.taintsNeedUpdate(tt.nodeTaints, tt.expectedTaints)

			assert.Equal(t, tt.expectUpdate, needsUpdate, "needsUpdate mismatch")
			assert.Equal(t, tt.expectedToAddCount, len(toAdd), "toAdd count mismatch")
			assert.Equal(t, tt.expectedToRemoveCount, len(toRemove), "toRemove count mismatch")
		})
	}
}

func TestApplyTaintChanges(t *testing.T) {
	h := &handler{}

	tests := []struct {
		name           string
		currentTaints  []corev1.Taint
		toAdd          []corev1.Taint
		toRemove       []int
		expectedTaints []corev1.Taint
	}{
		{
			name:          "add taint to empty list",
			currentTaints: nil,
			toAdd: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			toRemove: nil,
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		{
			name: "remove taint",
			currentTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			toAdd:          nil,
			toRemove:       []int{0},
			expectedTaints: []corev1.Taint{},
		},
		{
			name: "preserve user taints while removing default",
			currentTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				{Key: "custom-taint", Effect: corev1.TaintEffectNoSchedule},
			},
			toAdd:    nil,
			toRemove: []int{0},
			expectedTaints: []corev1.Taint{
				{Key: "custom-taint", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		{
			name: "add and remove",
			currentTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/etcd", Effect: corev1.TaintEffectNoExecute},
			},
			toAdd: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			toRemove: []int{0},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.applyTaintChanges(tt.currentTaints, tt.toAdd, tt.toRemove)

			assert.Equal(t, len(tt.expectedTaints), len(result), "result length mismatch")

			for i, expected := range tt.expectedTaints {
				assert.Equal(t, expected.Key, result[i].Key, "taint key mismatch at index %d", i)
				assert.Equal(t, expected.Effect, result[i].Effect, "taint effect mismatch at index %d", i)
			}
		})
	}
}

func TestWorkerLabelLogic(t *testing.T) {
	tests := []struct {
		name               string
		machineHasWorker   bool
		nodeHasWorker      bool
		shouldAddLabel     bool
		shouldRemoveLabel  bool
	}{
		{
			name:               "add worker label",
			machineHasWorker:   true,
			nodeHasWorker:      false,
			shouldAddLabel:     true,
			shouldRemoveLabel:  false,
		},
		{
			name:               "remove worker label",
			machineHasWorker:   false,
			nodeHasWorker:      true,
			shouldAddLabel:     false,
			shouldRemoveLabel:  true,
		},
		{
			name:               "no change - both have",
			machineHasWorker:   true,
			nodeHasWorker:      true,
			shouldAddLabel:     false,
			shouldRemoveLabel:  false,
		},
		{
			name:               "no change - neither have",
			machineHasWorker:   false,
			nodeHasWorker:      false,
			shouldAddLabel:     false,
			shouldRemoveLabel:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from reconcileNodeMetadata
			node := &corev1.Node{}
			if node.Labels == nil {
				node.Labels = make(map[string]string)
			}

			if tt.nodeHasWorker {
				node.Labels[workerLabel] = "true"
			}

			hasWorkerLabel := node.Labels[workerLabel] == "true"
			needsUpdate := false

			if tt.machineHasWorker && !hasWorkerLabel {
				// Add worker label
				node.Labels[workerLabel] = "true"
				needsUpdate = true
			} else if !tt.machineHasWorker && hasWorkerLabel {
				// Remove worker label
				delete(node.Labels, workerLabel)
				needsUpdate = true
			}

			if tt.shouldAddLabel || tt.shouldRemoveLabel {
				assert.True(t, needsUpdate, "expected update but needsUpdate was false")
			} else {
				assert.False(t, needsUpdate, "expected no update but needsUpdate was true")
			}

			// Verify final state
			if tt.machineHasWorker {
				assert.Equal(t, "true", node.Labels[workerLabel], "worker label should be present")
			} else {
				_, exists := node.Labels[workerLabel]
				assert.False(t, exists, "worker label should not be present")
			}
		})
	}
}
