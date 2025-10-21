package autoscaler

import (
	"fmt"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rke "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/buildconfig"
	"github.com/rancher/rancher/pkg/settings"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Test cases for ensureFleetHelmOp method

func (s *autoscalerSuite) TestEnsureFleetHelmOp_HappyPath_CreateNewHelmOp() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOp.EXPECT().Create(gomock.Any()).Do(func(helmOp *fleet.HelmOp) {
		s.Equal("default", helmOp.Namespace)
		s.Equal("autoscaler-default-test-cluster", helmOp.Name)
	}).Return(&fleet.HelmOp{}, nil)

	err := s.h.ensureFleetHelmOp(cluster, "v1", 3)
	s.NoError(err)
}

func (s *autoscalerSuite) TestEnsureFleetHelmOp_HappyPath_UpdateExistingHelmOp() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	existingHelmOp := &fleet.HelmOp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-autoscaler-test-cluster",
			Namespace: "default",
		},
		Spec: fleet.HelmOpSpec{
			BundleSpec: fleet.BundleSpec{
				Targets: []fleet.BundleTarget{},
			},
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(existingHelmOp, nil)
	s.helmOp.EXPECT().Update(gomock.Any()).Do(func(helmOp *fleet.HelmOp) {
		s.Equal("default", helmOp.Namespace)
		s.Equal("cluster-autoscaler-test-cluster", helmOp.Name)
	}).Return(&fleet.HelmOp{}, nil)

	err := s.h.ensureFleetHelmOp(cluster, "v1", 3)
	s.NoError(err)
}

func (s *autoscalerSuite) TestEnsureFleetHelmOp_HappyPath_NoUpdateNeeded() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	existingHelmOp := &fleet.HelmOp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "autoscaler-default-test-cluster",
			Namespace: "default",
		},
		Spec: fleet.HelmOpSpec{
			BundleSpec: fleet.BundleSpec{
				Targets: []fleet.BundleTarget{
					{
						ClusterName: "test-cluster",
					},
				},
				BundleDeploymentOptions: fleet.BundleDeploymentOptions{
					DefaultNamespace: "kube-system",
					Helm: &fleet.HelmOptions{
						Chart:       getChartName(),
						Version:     buildconfig.ClusterAutoscalerChartVersion,
						Repo:        settings.ClusterAutoscalerChartRepository.Get(),
						ReleaseName: "cluster-autoscaler",
						Values: &fleet.GenericMap{
							Data: map[string]any{
								"replicaCount": 3,
								"image":        s.h.getChartImageSettings(cluster),
								"autoDiscovery": map[string]any{
									"clusterName": cluster.Name,
									"namespace":   cluster.Namespace,
								},
								"cloudProvider":             "clusterapi",
								"clusterAPIMode":            "incluster-kubeconfig",
								"clusterAPICloudConfigPath": "/etc/kubernetes/mgmt-cluster/value",
								"extraVolumeSecrets": map[string]any{
									"local-cluster": map[string]any{
										"name":      "mgmt-kubeconfig",
										"mountPath": "/etc/kubernetes/mgmt-cluster",
									},
								},
								"extraArgs": map[string]any{
									"v": 2,
								},
								"extraEnv": map[string]any{
									"RANCHER_AUTOSCALER_KUBECONFIG_VERSION": "v1",
								},
							},
						},
					},
				},
			},
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(existingHelmOp, nil)
	s.helmOp.EXPECT().Update(gomock.Any()).Times(0)

	err := s.h.ensureFleetHelmOp(cluster, "v1", 3)
	s.NoError(err)
}

// Test cases for resolveImageTagVersion method

func (s *autoscalerSuite) TestResolveImageTagVersion_HappyPath_KnownVersion() {
	rkeCP := &rke.RKEControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RKEControlPlane",
			APIVersion: "rke.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: "default",
		},
		Spec: rke.RKEControlPlaneSpec{
			KubernetesVersion: "v1.34.0+k3s1",
		},
	}

	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "rke.cattle.io/v1",
				Kind:       "RKEControlPlane",
				Name:       "test-control-plane",
				Namespace:  "default",
			},
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		// Only return the RKEControlPlane if the GVK matches what we expect for RKEControlPlane
		if gvk.Group == "rke.cattle.io" && gvk.Version == "v1" && gvk.Kind == "RKEControlPlane" {
			return rkeCP, nil
		}
		return nil, fmt.Errorf("not found")
	})

	result := s.h.resolveImageTagVersion(cluster)
	s.Equal("1.34.0-3.4", result) // Should return the version for minor 34
}

func (s *autoscalerSuite) TestResolveImageTagVersion_EdgeCase_UnknownVersion() {
	// Temporarily modify the imageTagVersions map to test unknown version
	originalMap := imageTagVersions
	imageTagVersions = map[int]string{
		99: "1.99.0-9.9", // Use a version that's not in the original map
	}

	defer func() {
		imageTagVersions = originalMap // Restore original map
	}()

	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	result := s.h.resolveImageTagVersion(cluster)
	s.Equal("", result) // Should return empty string for unknown version
}

// Test cases for getChartImageSettings method

func (s *autoscalerSuite) TestGetChartImageSettings_HappyPath_NoOverride() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	result := s.h.getChartImageSettings(cluster)
	s.Equal(map[string]any{}, result) // Should return empty map when no override
}

func (s *autoscalerSuite) TestGetChartImageSettings_HappyPath_WithValidImage() {
	originalImage := settings.ClusterAutoscalerImage.Get()
	defer settings.ClusterAutoscalerImage.Set(originalImage)
	settings.ClusterAutoscalerImage.Set("registry.example.com/cluster-autoscaler:1.2.3")

	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	result := s.h.getChartImageSettings(cluster)
	expected := map[string]any{
		"repository": "cluster-autoscaler",
		"registry":   "registry.example.com",
		"tag":        "1.2.3",
	}
	s.Equal(expected, result)
}

// Test cases for getKubernetesMinorVersion method

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_HappyPath_RKEControlPlane() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "rke.cattle.io/v1",
				Kind:       "RKEControlPlane",
				Name:       "test-control-plane",
				Namespace:  "default",
			},
		},
	}

	rkeCP := &rke.RKEControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RKEControlPlane",
			APIVersion: "rke.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: "default",
		},
		Spec: rke.RKEControlPlaneSpec{
			KubernetesVersion: "v1.34.0+k3s1",
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		// Only return the RKEControlPlane if the GVK matches what we expect for RKEControlPlane
		if gvk.Group == "rke.cattle.io" && gvk.Version == "v1" && gvk.Kind == "RKEControlPlane" {
			return rkeCP, nil
		}
		return nil, fmt.Errorf("not found")
	})

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(34, result) // Should return minor version 34
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_HappyPath_CAPIControlPlane() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "controlplane.cluster.x-k8s.io/v1beta1",
				Kind:       "KubeadmControlPlane",
				Name:       "test-control-plane",
				Namespace:  "default",
			},
		},
	}

	unstructuredCP := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"version": "v1.33.0+k3s1",
			},
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		// Only return the Unstructured object if the GVK matches what we expect for CAPI control plane
		if gvk.Group == "controlplane.cluster.x-k8s.io" && gvk.Version == "v1beta1" && gvk.Kind == "KubeadmControlPlane" {
			return unstructuredCP, nil
		}
		return nil, fmt.Errorf("not found")
	})

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(33, result) // Should return minor version 33
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_EdgeCase_ErrorGettingControlPlane() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "rke.cattle.io/v1",
				Kind:       "RKEControlPlane",
				Name:       "non-existent",
				Namespace:  "default",
			},
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		// Only return an error if the GVK matches what we expect for RKEControlPlane
		if gvk.Group == "rke.cattle.io" && gvk.Version == "v1" && gvk.Kind == "RKEControlPlane" {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("not found")
	})

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(0, result) // Should return 0 on error
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_EdgeCase_InvalidVersion() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "rke.cattle.io/v1",
				Kind:       "RKEControlPlane",
				Name:       "test-control-plane",
				Namespace:  "default",
			},
		},
	}

	rkeCP := &rke.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: "default",
		},
		Spec: rke.RKEControlPlaneSpec{
			KubernetesVersion: "invalid-version",
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		// Only return the RKEControlPlane if the GVK matches what we expect for RKEControlPlane
		if gvk.Group == "rke.cattle.io" && gvk.Version == "v1" && gvk.Kind == "RKEControlPlane" {
			return rkeCP, nil
		}
		return nil, fmt.Errorf("not found")
	})

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(0, result) // Should return 0 for invalid version
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_EdgeCase_NilControlPlaneRef() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: nil, // Nil control plane ref
		},
	}

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(0, result) // Should return 0 when ControlPlaneRef is nil
}

// Test cases for cleanupFleet method

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_HelmOpExists() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(&fleet.HelmOp{}, nil)
	s.helmOp.EXPECT().Delete("default", "autoscaler-default-test-cluster", &metav1.DeleteOptions{}).Return(nil)

	err := s.h.cleanupFleet(cluster)
	s.NoError(err)
}

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_HelmOpDoesNotExist() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	err := s.h.cleanupFleet(cluster)
	s.NoError(err)
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_DeleteError() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(&fleet.HelmOp{}, nil)
	s.helmOp.EXPECT().Delete("default", "autoscaler-default-test-cluster", &metav1.DeleteOptions{}).Return(fmt.Errorf("delete failed"))

	err := s.h.cleanupFleet(cluster)
	s.Error(err)
	s.Contains(err.Error(), "failed to delete Helm operation")
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_GetError() {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	s.helmOpCache.EXPECT().Get("default", "autoscaler-default-test-cluster").Return(nil, fmt.Errorf("get failed"))

	err := s.h.cleanupFleet(cluster)
	s.Error(err)
	s.Contains(err.Error(), "failed to check existence of Helm operation")
}

// Test cases for cleanupFleet method

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_SuccessfulHelmOpDeletion() {
	// Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	helmOpName := helmOpName(cluster)

	// Set up mock expectations for successful HelmOp deletion
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(&fleet.HelmOp{}, nil)
	s.helmOp.EXPECT().Delete(cluster.Namespace, helmOpName, &metav1.DeleteOptions{}).Return(nil)

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result
	s.NoError(err, "Expected no error when successfully cleaning up HelmOp")
}

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_NoHelmOpExists() {
	// Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	helmOpName := helmOpName(cluster)

	// Set up mock expectations for HelmOp not found
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, errors.NewNotFound(schema.GroupResource{}, "helmop"))

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result - should succeed when HelmOp doesn't exist
	s.NoError(err, "Expected no error when HelmOp doesn't exist")
}

func (s *autoscalerSuite) TestCleanupFleet_Error_FailedToDeleteHelmOp() {
	// Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	helmOpName := helmOpName(cluster)
	deleteError := fmt.Errorf("failed to delete HelmOp: access denied")

	// Set up mock expectations
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(&fleet.HelmOp{}, nil)
	s.helmOp.EXPECT().Delete(cluster.Namespace, helmOpName, &metav1.DeleteOptions{}).Return(deleteError)

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result
	s.Error(err, "Expected error when HelmOp deletion fails")
	s.Contains(err.Error(), "encountered 1 errors during fleet cleanup", "Error should mention count of errors")
	s.Contains(err.Error(), "failed to delete Helm operation "+helmOpName+" in namespace "+cluster.Namespace, "Error should include HelmOp name and namespace")
	s.Contains(err.Error(), "access denied", "Original error should be preserved")
}

func (s *autoscalerSuite) TestCleanupFleet_Error_FailedToCheckHelmOpExistence() {
	// Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	helmOpName := helmOpName(cluster)
	checkError := fmt.Errorf("failed to check HelmOp existence: network timeout")

	// Set up mock expectations for cache error
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, checkError)

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result
	s.Error(err, "Expected error when HelmOp existence check fails")
	s.Contains(err.Error(), "encountered 1 errors during fleet cleanup", "Error should mention count of errors")
	s.Contains(err.Error(), "failed to check existence of Helm operation "+helmOpName+" in namespace "+cluster.Namespace, "Error should include HelmOp name and namespace")
	s.Contains(err.Error(), "network timeout", "Original error should be preserved")
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_ClusterWithEmptyName() {
	// Create test cluster with empty name
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "",
			Namespace: "default",
		},
	}

	helmOpName := helmOpName(cluster)

	// Set up mock expectations for HelmOp not found (should handle empty names gracefully)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, errors.NewNotFound(schema.GroupResource{}, "helmop"))

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result
	s.NoError(err, "Expected no error when cluster has empty name")
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_ClusterWithEmptyNamespace() {
	// Create test cluster with empty namespace
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "",
		},
	}

	helmOpName := helmOpName(cluster)

	// Set up mock expectations for HelmOp not found (should handle empty namespace gracefully)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, errors.NewNotFound(schema.GroupResource{}, "helmop"))

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result
	s.NoError(err, "Expected no error when cluster has empty namespace")
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_ClusterWithSpecialCharacters() {
	// Create test cluster with special characters in name and namespace
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-123",
			Namespace: "test-namespace-456",
		},
	}

	helmOpName := helmOpName(cluster)

	// Set up mock expectations for successful deletion (should handle special characters gracefully)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(&fleet.HelmOp{}, nil)
	s.helmOp.EXPECT().Delete(cluster.Namespace, helmOpName, &metav1.DeleteOptions{}).Return(nil)

	// Call the method
	err := s.h.cleanupFleet(cluster)

	// Assert the result
	s.NoError(err, "Expected no error when cluster has special characters in name and namespace")
}
