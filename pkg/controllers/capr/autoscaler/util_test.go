package autoscaler

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestAutoscalerUserName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		expected string
	}{
		{
			name: "basic cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: "test-cluster-autoscaler",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-123",
				},
			},
			expected: "test-cluster-123-autoscaler",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			expected: "-autoscaler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := autoscalerUserName(tt.cluster)
			if result != tt.expected {
				t.Errorf("autoscalerUserName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGlobalRoleName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		expected string
	}{
		{
			name: "basic cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: "test-cluster-autoscaler-global-role",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-cluster-2023",
				},
			},
			expected: "prod-cluster-2023-autoscaler-global-role",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			expected: "-autoscaler-global-role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := globalRoleName(tt.cluster)
			if result != tt.expected {
				t.Errorf("globalRoleName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGlobalRoleBindingName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		expected string
	}{
		{
			name: "basic cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: "test-cluster-autoscaler-global-rolebinding",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "staging-cluster-v2",
				},
			},
			expected: "staging-cluster-v2-autoscaler-global-rolebinding",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			expected: "-autoscaler-global-rolebinding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := globalRoleBindingName(tt.cluster)
			if result != tt.expected {
				t.Errorf("globalRoleBindingName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestOwnerReference(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		expected []metav1.OwnerReference
	}{
		{
			name: "basic cluster",
			cluster: &capi.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					UID:  "test-uid-123",
				},
			},
			expected: []metav1.OwnerReference{{
				APIVersion:         "provisioning.cattle.io/v1",
				Kind:               "Cluster",
				Name:               "test-cluster",
				UID:                "test-uid-123",
				Controller:         &[]bool{true}[0],
				BlockOwnerDeletion: &[]bool{true}[0],
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ownerReference(tt.cluster)
			if len(result) != len(tt.expected) {
				t.Errorf("ownerReference() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i].APIVersion != tt.expected[i].APIVersion ||
					result[i].Kind != tt.expected[i].Kind ||
					result[i].Name != tt.expected[i].Name ||
					result[i].UID != tt.expected[i].UID ||
					*result[i].Controller != *tt.expected[i].Controller ||
					*result[i].BlockOwnerDeletion != *tt.expected[i].BlockOwnerDeletion {
					t.Errorf("ownerReference()[%d] = %+v, want %+v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestKubeconfigSecretName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		expected string
	}{
		{
			name: "basic cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expected: "test-cluster-autoscaler-kubeconfig",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-cluster-2023",
				},
			},
			expected: "prod-cluster-2023-autoscaler-kubeconfig",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			expected: "-autoscaler-kubeconfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kubeconfigSecretName(tt.cluster)
			if result != tt.expected {
				t.Errorf("kubeconfigSecretName() = %v, want %v", result, tt.expected)
			}
		})
	}
}
