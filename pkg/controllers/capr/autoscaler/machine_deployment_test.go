package autoscaler

import (
	"fmt"

	"github.com/rancher/rancher/pkg/capr"
	"go.uber.org/mock/gomock"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_ReplicaCountHigherThanMachinePoolQuantity_ShouldIncreaseQuantity() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(5),
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "worker-pool",
						Quantity: int32Ptr(3),
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(capiCluster, nil)
	s.clusterCache.EXPECT().Get("default", "test-prov-cluster").Return(provisioningCluster, nil)
	s.clusterClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *provv1.Cluster) (*provv1.Cluster, error) {
		// Verify the cluster was updated with the correct quantity
		s.NotNil(cluster.Spec.RKEConfig.MachinePools[0].Quantity)
		s.Equal(int32(5), *cluster.Spec.RKEConfig.MachinePools[0].Quantity)
		return cluster, nil
	})

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
	s.Equal(int32(5), *result.Spec.Replicas)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_ReplicaCountSameAsMachinePoolQuantity_ShouldNotDoAnything() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(3),
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "worker-pool",
						Quantity: int32Ptr(3),
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(capiCluster, nil)
	s.clusterCache.EXPECT().Get("default", "test-prov-cluster").Return(provisioningCluster, nil)
	// No update should be called since quantities are the same

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
	s.Equal(int32(3), *result.Spec.Replicas)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_ReplicaCountLowerThanMachinePoolQuantity_ShouldReduceQuantity() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(2),
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "worker-pool",
						Quantity: int32Ptr(5),
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(capiCluster, nil)
	s.clusterCache.EXPECT().Get("default", "test-prov-cluster").Return(provisioningCluster, nil)
	s.clusterClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *provv1.Cluster) (*provv1.Cluster, error) {
		// Verify the cluster was updated with the correct quantity
		s.NotNil(cluster.Spec.RKEConfig.MachinePools[0].Quantity)
		s.Equal(int32(2), *cluster.Spec.RKEConfig.MachinePools[0].Quantity)
		return cluster, nil
	})

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
	s.Equal(int32(2), *result.Spec.Replicas)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_NilMachineDeployment() {
	// Setup test data
	var machineDeployment *capi.MachineDeployment = nil

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.Nil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MachineDeploymentWithDeletionTimestamp() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-md",
			Namespace:         "default",
			DeletionTimestamp: &metav1.Time{},
		},
	}

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MissingClusterNameLabel() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Labels: map[string]string{
				capr.RKEMachinePoolNameLabel: "worker-pool",
			},
		},
	}

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MissingMachinePoolNameLabel() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel: "test-cluster",
					},
				},
			},
		},
	}

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_CAPIClusterNotFound() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(nil, fmt.Errorf("cluster not found"))

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.Error(err)
	s.Nil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_NoMachinePools() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(5),
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(capiCluster, nil)
	// The GetProvisioningClusterFromCAPICluster function will call clusterCache.Get internally
	// to get the provisioning cluster based on the owner reference
	s.clusterCache.EXPECT().Get("default", "test-prov-cluster").Return(provisioningCluster, nil)

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MatchingMachinePoolNotFound() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Labels: map[string]string{
				capi.ClusterNameLabel:        "test-cluster",
				capr.RKEMachinePoolNameLabel: "worker-pool",
			},
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(5),
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "different-pool",
						Quantity: int32Ptr(3),
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(capiCluster, nil).AnyTimes()
	s.clusterCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(provisioningCluster, nil).AnyTimes()

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MachinePoolQuantityNil() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Labels: map[string]string{
				capi.ClusterNameLabel:        "test-cluster",
				capr.RKEMachinePoolNameLabel: "worker-pool",
			},
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(5),
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "worker-pool",
						Quantity: nil,
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(capiCluster, nil).AnyTimes()
	s.clusterCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(provisioningCluster, nil).AnyTimes()

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MachineDeploymentReplicasNil() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
			Replicas: nil,
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "worker-pool",
						Quantity: int32Ptr(3),
					},
				},
			},
		},
	}

	// Setup mock expectations
	s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(capiCluster, nil)
	s.clusterCache.EXPECT().Get("default", "test-prov-cluster").Return(provisioningCluster, nil)

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func (s *autoscalerSuite) TestSyncMachinePoolReplicas_EdgeCase_MultipleMachinePools() {
	// Setup test data
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
		},
		Spec: capi.MachineDeploymentSpec{
			Replicas: int32Ptr(5),
			Template: capi.MachineTemplateSpec{
				ObjectMeta: capi.ObjectMeta{
					Labels: map[string]string{
						capi.ClusterNameLabel:        "test-cluster",
						capr.RKEMachinePoolNameLabel: "worker-pool",
					},
				},
			},
		},
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
					Name:       "test-prov-cluster",
				},
			},
		},
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prov-cluster",
			Namespace: "default",
		},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				MachinePools: []provv1.RKEMachinePool{
					{
						Name:     "control-plane-pool",
						Quantity: int32Ptr(3),
					},
					{
						Name:     "worker-pool",
						Quantity: int32Ptr(2),
					},
					{
						Name:     "gpu-pool",
						Quantity: int32Ptr(1),
					},
				},
			},
		},
	}

	// Setup mock expectations in order
	gomock.InOrder(
		s.capiClusterCache.EXPECT().Get("default", "test-cluster").Return(capiCluster, nil),
		s.clusterCache.EXPECT().Get("default", "test-prov-cluster").Return(provisioningCluster, nil),
		s.clusterClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *provv1.Cluster) (*provv1.Cluster, error) {
			// Verify only the matching machine pool was updated
			s.NotNil(cluster.Spec.RKEConfig.MachinePools[1].Quantity)
			s.Equal(int32(5), *cluster.Spec.RKEConfig.MachinePools[1].Quantity)
			// Verify other pools remain unchanged
			s.Equal(int32(3), *cluster.Spec.RKEConfig.MachinePools[0].Quantity)
			s.Equal(int32(1), *cluster.Spec.RKEConfig.MachinePools[2].Quantity)
			return cluster, nil
		}),
	)

	// Execute test
	result, err := s.m.syncMachinePoolReplicas("", machineDeployment)

	// Verify results
	s.NoError(err)
	s.NotNil(result)
}

func int32Ptr(i int32) *int32 {
	return &i
}
