package autoscaler

import (
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			expected: "default-test-cluster-autoscaler",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-123",
					Namespace: "production",
				},
			},
			expected: "production-test-cluster-123-autoscaler",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "test-ns",
				},
			},
			expected: "test-ns--autoscaler",
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
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			expected: "default-test-cluster-autoscaler-global-role",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prod-cluster-2023",
					Namespace: "production",
				},
			},
			expected: "production-prod-cluster-2023-autoscaler-global-role",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "test-ns",
				},
			},
			expected: "test-ns--autoscaler-global-role",
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
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			expected: "default-test-cluster-autoscaler-global-rolebinding",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-cluster-v2",
					Namespace: "staging",
				},
			},
			expected: "staging-staging-cluster-v2-autoscaler-global-rolebinding",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "test-ns",
				},
			},
			expected: "test-ns--autoscaler-global-rolebinding",
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
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ownerReference() = %+v, want %+v", result, tt.expected)
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
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			expected: "default-test-cluster-autoscaler-kubeconfig",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prod-cluster-2023",
					Namespace: "production",
				},
			},
			expected: "production-prod-cluster-2023-autoscaler-kubeconfig",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "test-ns",
				},
			},
			expected: "test-ns--autoscaler-kubeconfig",
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

func TestHelmOpName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		expected string
	}{
		{
			name: "basic cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			expected: "autoscaler-default-test-cluster",
		},
		{
			name: "cluster with special characters",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prod-cluster-2023",
					Namespace: "production",
				},
			},
			expected: "autoscaler-production-prod-cluster-2023",
		},
		{
			name: "empty cluster name",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "test-ns",
				},
			},
			expected: "autoscaler-test-ns-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helmOpName(tt.cluster)
			if result != tt.expected {
				t.Errorf("helmOpName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAutoScalerChartRepositoryHost(t *testing.T) {
	tests := []struct {
		name     string
		setting  string
		expected string
	}{
		{
			name:     "full URL with https and path",
			setting:  "https://charts.example.com/autoscaler/charts",
			expected: "charts.example.com",
		},
		{
			name:     "OCI URL with https and path",
			setting:  "oci://charts.example.com/autoscaler/charts",
			expected: "charts.example.com",
		},
		{
			name:     "full URL with http and path",
			setting:  "http://charts.example.com/some/path",
			expected: "charts.example.com",
		},
		{
			name:     "URL with no path",
			setting:  "https://charts.example.com",
			expected: "charts.example.com",
		},
		{
			name:     "host only, no protocol",
			setting:  "charts.example.com",
			expected: "charts.example.com",
		},
		{
			name:     "host with path, no protocol",
			setting:  "charts.example.com/some/path",
			expected: "charts.example.com",
		},
		{
			name:     "empty setting",
			setting:  "",
			expected: "",
		},
		{
			name:     "host with port and path",
			setting:  "https://registry.example.com:5000/helm/charts",
			expected: "registry.example.com:5000",
		},
		{
			name:     "host with port, no path",
			setting:  "https://registry.example.com:5000",
			expected: "registry.example.com:5000",
		},
		{
			name:     "oci protocol",
			setting:  "oci://registry.example.com/charts",
			expected: "registry.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := settings.ClusterAutoscalerChartRepository.Get()
			settings.ClusterAutoscalerChartRepository.Set(tt.setting)
			t.Cleanup(func() {
				settings.ClusterAutoscalerChartRepository.Set(prev)
			})

			result := autoScalerChartRepositoryHost()
			if result != tt.expected {
				t.Errorf("autoScalerChartRepositoryHost() = %q, want %q", result, tt.expected)
			}
		})
	}
}
