package capr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestGetExpectedDefaultTaints(t *testing.T) {
	tests := []struct {
		name           string
		machineLabels  map[string]string
		runtime        string
		expectedTaints []corev1.Taint
	}{
		{
			name: "worker role only - no taints",
			machineLabels: map[string]string{
				WorkerRoleLabel: "true",
			},
			runtime:        RuntimeRKE2,
			expectedTaints: nil,
		},
		{
			name: "control plane + worker - no taints",
			machineLabels: map[string]string{
				ControlPlaneRoleLabel: "true",
				WorkerRoleLabel:       "true",
			},
			runtime:        RuntimeRKE2,
			expectedTaints: nil,
		},
		{
			name: "control plane only (RKE2) - control-plane taint",
			machineLabels: map[string]string{
				ControlPlaneRoleLabel: "true",
			},
			runtime: RuntimeRKE2,
			expectedTaints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
		{
			name: "etcd only (RKE2) - etcd taint",
			machineLabels: map[string]string{
				EtcdRoleLabel: "true",
			},
			runtime: RuntimeRKE2,
			expectedTaints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/etcd",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
		},
		{
			name: "control plane + etcd (RKE2) - both taints",
			machineLabels: map[string]string{
				ControlPlaneRoleLabel: "true",
				EtcdRoleLabel:         "true",
			},
			runtime: RuntimeRKE2,
			expectedTaints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/etcd",
					Effect: corev1.TaintEffectNoExecute,
				},
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
		{
			name: "control plane + etcd (K3s) - only control-plane taint",
			machineLabels: map[string]string{
				ControlPlaneRoleLabel: "true",
				EtcdRoleLabel:         "true",
			},
			runtime: RuntimeK3S,
			expectedTaints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
		{
			name: "all roles - no taints (worker role present)",
			machineLabels: map[string]string{
				ControlPlaneRoleLabel: "true",
				EtcdRoleLabel:         "true",
				WorkerRoleLabel:       "true",
			},
			runtime:        RuntimeRKE2,
			expectedTaints: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tt.machineLabels,
				},
			}

			taints := GetExpectedDefaultTaints(machine, tt.runtime)

			assert.Equal(t, len(tt.expectedTaints), len(taints), "taint count mismatch")

			// Check that all expected taints are present
			for _, expectedTaint := range tt.expectedTaints {
				found := false
				for _, taint := range taints {
					if taint.Key == expectedTaint.Key && taint.Effect == expectedTaint.Effect {
						found = true
						break
					}
				}
				assert.True(t, found, "expected taint not found: %s:%s", expectedTaint.Key, expectedTaint.Effect)
			}
		})
	}
}

func TestIsDefaultTaint(t *testing.T) {
	tests := []struct {
		name      string
		taint     corev1.Taint
		isDefault bool
	}{
		{
			name: "control-plane taint is default",
			taint: corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Effect: corev1.TaintEffectNoSchedule,
			},
			isDefault: true,
		},
		{
			name: "etcd taint is default",
			taint: corev1.Taint{
				Key:    "node-role.kubernetes.io/etcd",
				Effect: corev1.TaintEffectNoExecute,
			},
			isDefault: true,
		},
		{
			name: "custom taint is not default",
			taint: corev1.Taint{
				Key:    "custom-taint",
				Effect: corev1.TaintEffectNoSchedule,
			},
			isDefault: false,
		},
		{
			name: "control-plane with wrong effect is not default",
			taint: corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Effect: corev1.TaintEffectNoExecute,
			},
			isDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDefaultTaint(tt.taint)
			assert.Equal(t, tt.isDefault, result)
		})
	}
}

func TestParseTaintsAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    []corev1.Taint
		expectError bool
	}{
		{
			name:        "no annotation",
			annotations: map[string]string{},
			expected:    nil,
			expectError: false,
		},
		{
			name: "empty annotation",
			annotations: map[string]string{
				TaintsAnnotation: "",
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "valid taints",
			annotations: map[string]string{
				TaintsAnnotation: `[{"key":"custom-taint","effect":"NoSchedule"}]`,
			},
			expected: []corev1.Taint{
				{
					Key:    "custom-taint",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectError: false,
		},
		{
			name: "invalid json",
			annotations: map[string]string{
				TaintsAnnotation: "invalid-json",
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taints, err := ParseTaintsAnnotation(tt.annotations)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, taints)
			}
		})
	}
}
