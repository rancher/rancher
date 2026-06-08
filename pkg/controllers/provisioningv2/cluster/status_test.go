package cluster

import (
	"fmt"
	"testing"

	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestGetMachineProvider(t *testing.T) {
	h := &handler{}

	tests := []struct {
		name        string
		cluster     *v3.Cluster
		provCluster *v1.Cluster
		want        string
	}{
		{
			name: "Provisioning Cluster with UI annotation",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"ui.rancher/provider": "custom-provider",
					},
				},
			},
			want: "custom-provider",
		},
		{
			name: "Custom v2prov Cluster",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			provCluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{},
					},
				},
			},
			want: "custom",
		},
		{
			name: "Node Driver provisioned v2prov cluster",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			provCluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								NodeConfig: &corev1.ObjectReference{
									Kind: "Amazonec2Config",
									Name: "nc-cluster-pool1-lj66m",
								},
							},
						},
					},
				},
			},
			want: "Amazonec2",
		},
		{
			name: "Harvester Label",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"provider.cattle.io": "harvester",
					},
				},
			},
			provCluster: nil,
			want:        "harvester",
		},
		{
			name: "Harvester Status Provider",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Provider: "harvester",
				},
			},
			provCluster: nil,
			want:        "harvester",
		},
		{
			name: "Local Cluster",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					Internal: true,
				},
			},
			provCluster: nil,
			want:        "local",
		},
		{
			name: "AKS Hosted",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{
						Imported: false,
					},
				},
			},
			provCluster: nil,
			want:        "aks",
		},
		{
			name: "AKS Imported",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{
						Imported: true,
					},
				},
			},
			provCluster: nil,
			want:        "imported",
		},
		{
			name: "EKS Hosted",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					EKSConfig: &eksv1.EKSClusterConfigSpec{
						Imported: false,
					},
				},
			},
			provCluster: nil,
			want:        "eks",
		},
		{
			name: "GKE Hosted",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					GKEConfig: &gkev1.GKEClusterConfigSpec{
						Imported: false,
					},
				},
			},
			provCluster: nil,
			want:        "gke",
		},
		{
			name: "Ali Hosted",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AliConfig: &aliv1.AliClusterConfigSpec{
						Imported: false,
					},
				},
			},
			provCluster: nil,
			want:        "ali",
		},
		{
			name: "Generic Engine (Oracle)",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					GenericEngineConfig: &v3.MapStringInterface{
						"driverName": "oracle",
					},
				},
			},
			provCluster: nil,
			want:        "oracle",
		},
		{
			name: "Generic Imported Cluster",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Driver: "",
				},
			},
			provCluster: nil,
			want:        "imported",
		},
		{
			name: "RKE2/K3s Imported Cluster",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Driver:   "rke2",
					Provider: "rke2",
				},
			},
			provCluster: nil,
			want:        "imported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.getMachineProvider(tt.cluster, tt.provCluster)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetKubernetesVersion(t *testing.T) {
	h := &handler{}
	k8sVersion := "v1.21.1"

	tests := []struct {
		name        string
		cluster     *v3.Cluster
		provCluster *v1.Cluster
		want        string
	}{
		{
			name: "Status GitVersion Present",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{
						GitVersion: k8sVersion,
					},
				},
			},
			provCluster: nil,
			want:        k8sVersion,
		},
		{
			name: "Provisioning cluster with empty status version",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{},
				},
			},
			provCluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig:         &v1.RKEConfig{},
					KubernetesVersion: "v1.21.0+rke2r1",
				},
			},
			want: "v1.21.0+rke2r1",
		},
		{
			name: "AKS Version",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{},
				},
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{
						KubernetesVersion: &k8sVersion,
					},
				},
			},
			provCluster: nil,
			want:        k8sVersion,
		},
		{
			name: "EKS Version",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{},
				},
				Spec: v3.ClusterSpec{
					EKSConfig: &eksv1.EKSClusterConfigSpec{
						KubernetesVersion: &k8sVersion,
					},
				},
			},
			provCluster: nil,
			want:        "v1.21.1",
		},
		{
			name: "Generic Engine Version",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{},
				},
				Spec: v3.ClusterSpec{
					GenericEngineConfig: &v3.MapStringInterface{
						"kubernetesVersion": "v1.19.0",
					},
				},
			},
			provCluster: nil,
			want:        "v1.19.0",
		},
		{
			name: "Empty",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{},
				},
			},
			provCluster: nil,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.getKubernetesVersion(tt.cluster, tt.provCluster)
			assert.Equal(t, tt.want, got)
		})
	}
}

// mockMachineCache implements capicontrollers.MachineCache for testing.
type mockMachineCache struct {
	machines []*capiv1beta2.Machine
	err      error
}

func (m *mockMachineCache) Get(namespace, name string) (*capiv1beta2.Machine, error) {
	return nil, nil
}

func (m *mockMachineCache) List(namespace string, selector labels.Selector) ([]*capiv1beta2.Machine, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*capiv1beta2.Machine
	for _, machine := range m.machines {
		if machine.Namespace == namespace && selector.Matches(labels.Set(machine.Labels)) {
			result = append(result, machine)
		}
	}
	return result, nil
}

func (m *mockMachineCache) AddIndexer(indexName string, indexer generic.Indexer[*capiv1beta2.Machine]) {
}

func (m *mockMachineCache) GetByIndex(indexName, key string) ([]*capiv1beta2.Machine, error) {
	return nil, nil
}

func TestGetNodeCount(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *v3.Cluster
		provCluster *v1.Cluster
		cache       *mockMachineCache
		wantCount   int
		wantErr     bool
	}{
		{
			name: "v2prov cluster uses capi machines cache",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 1, // stale count in status
				},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
				},
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{},
				},
			},
			cache: &mockMachineCache{
				machines: []*capiv1beta2.Machine{
					{ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "fleet-default", Labels: map[string]string{capiv1beta2.ClusterNameLabel: "test-cluster"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "m2", Namespace: "fleet-default", Labels: map[string]string{capiv1beta2.ClusterNameLabel: "test-cluster"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "m3", Namespace: "fleet-default", Labels: map[string]string{capiv1beta2.ClusterNameLabel: "test-cluster"}}},
				},
			},
			wantCount: 3,
		},
		{
			name: "v2prov cluster with zero machines",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 5,
				},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
				},
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{},
				},
			},
			cache:     &mockMachineCache{},
			wantCount: 0,
		},
		{
			name: "v2prov cluster cache filters by cluster name label",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 1,
				},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "fleet-default",
				},
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{},
				},
			},
			cache: &mockMachineCache{
				machines: []*capiv1beta2.Machine{
					{ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "fleet-default", Labels: map[string]string{capiv1beta2.ClusterNameLabel: "my-cluster"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "m2", Namespace: "fleet-default", Labels: map[string]string{capiv1beta2.ClusterNameLabel: "other-cluster"}}},
				},
			},
			wantCount: 1,
		},
		{
			name: "v2prov cluster cache error returns error",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 5,
				},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
				},
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{},
				},
			},
			cache:   &mockMachineCache{err: fmt.Errorf("cache unavailable")},
			wantErr: true,
		},
		{
			name: "non-v2prov cluster falls back to status.NodeCount",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 7,
				},
			},
			provCluster: nil,
			cache:       &mockMachineCache{},
			wantCount:   7,
		},
		{
			name: "provCluster without RKEConfig falls back to status.NodeCount",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 4,
				},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imported-cluster",
					Namespace: "fleet-default",
				},
				Spec: v1.ClusterSpec{},
			},
			cache:     &mockMachineCache{},
			wantCount: 4,
		},
		{
			name: "v2prov cluster with nil capiMachinesCache falls back to status.NodeCount",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					NodeCount: 2,
				},
			},
			provCluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
				},
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{},
				},
			},
			cache:     nil, // nil cache
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				capiMachinesCache: tt.cache,
			}
			// A nil mock means we want capiMachinesCache to truly be nil on the handler.
			if tt.cache == nil {
				h.capiMachinesCache = nil
			}
			got, err := h.getNodeCount(tt.cluster, tt.provCluster)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, got)
		})
	}
}

func TestGetKubernetesProvider(t *testing.T) {
	h := &handler{}

	tests := []struct {
		name                   string
		cluster                *v3.Cluster
		provisioningClusterRef *corev1.ObjectReference
		kubernetesVersion      string
		want                   string
	}{
		{
			name:                   "v2prov rke2 cluster",
			cluster:                &v3.Cluster{},
			provisioningClusterRef: &corev1.ObjectReference{Name: "test-cluster", Namespace: "fleet-default"},
			kubernetesVersion:      "v1.28.10+rke2r1",
			want:                   "rke2",
		},
		{
			name:                   "v2prov k3s cluster",
			cluster:                &v3.Cluster{},
			provisioningClusterRef: &corev1.ObjectReference{Name: "test-cluster", Namespace: "fleet-default"},
			kubernetesVersion:      "v1.28.10+k3s1",
			want:                   "k3s",
		},
		{
			name:                   "non-v2prov cluster does not set provider early",
			cluster:                &v3.Cluster{},
			provisioningClusterRef: nil,
			kubernetesVersion:      "v1.28.10+rke2r1",
			want:                   "",
		},
		{
			name: "already set provider is preserved",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Provider: "harvester",
				},
			},
			provisioningClusterRef: &corev1.ObjectReference{Name: "test-cluster", Namespace: "fleet-default"},
			kubernetesVersion:      "v1.28.10+rke2r1",
			want:                   "harvester",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.getKubernetesProvider(tt.cluster, tt.provisioningClusterRef, tt.kubernetesVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}
