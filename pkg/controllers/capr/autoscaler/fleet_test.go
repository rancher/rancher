package autoscaler

import (
	"fmt"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rke "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/buildconfig"
	"github.com/rancher/rancher/pkg/settings"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// Test cases for ensureFleetHelmOp method
// Test helper functions

func (s *autoscalerSuite) createTestCluster(name, namespace string) *capi.Cluster {
	return &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (s *autoscalerSuite) createTestClusterWithControlPlane(name, namespace, apiGroup, kind, cpName string) *capi.Cluster {
	cluster := s.createTestCluster(name, namespace)
	cluster.Spec.ControlPlaneRef = capi.ContractVersionedObjectReference{
		APIGroup: apiGroup,
		Kind:     kind,
		Name:     cpName,
	}
	return cluster
}

func (s *autoscalerSuite) setupDiscoveryForAPIGroup(apiGroup, version, kind string) {
	s.discovery.serverGroupsFunc = func() (*metav1.APIGroupList, error) {
		return &metav1.APIGroupList{
			Groups: []metav1.APIGroup{
				{
					Name: apiGroup,
					PreferredVersion: metav1.GroupVersionForDiscovery{
						Version: version,
					},
				},
			},
		}, nil
	}

	s.discovery.serverResourcesForGroupVersionFunc = func(groupVersion string) (*metav1.APIResourceList, error) {
		if groupVersion == apiGroup+"/"+version {
			return &metav1.APIResourceList{
				APIResources: []metav1.APIResource{
					{Kind: kind},
				},
			}, nil
		}
		return nil, fmt.Errorf("not found")
	}
}

func (s *autoscalerSuite) setupDynamicClientForRKEControlPlane(kubernetesVersion string) {
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
			KubernetesVersion: kubernetesVersion,
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		if gvk.Group == "rke.cattle.io" && gvk.Kind == "RKEControlPlane" {
			return rkeCP, nil
		}
		return nil, fmt.Errorf("not found")
	})
}

func (s *autoscalerSuite) setupDynamicClientForCAPIControlPlane(kubernetesVersion string) {
	unstructuredCP := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"version": kubernetesVersion,
			},
		},
	}

	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		if gvk.Group == "controlplane.cluster.x-k8s.io" && gvk.Kind == "KubeadmControlPlane" {
			return unstructuredCP, nil
		}
		return nil, fmt.Errorf("not found")
	})
}

func (s *autoscalerSuite) expectHelmOpGet(namespace, name string, helmOp *fleet.HelmOp, err error) {
	s.helmOpCache.EXPECT().Get(namespace, name).Return(helmOp, err)
}

func (s *autoscalerSuite) expectHelmOpCreate() {
	s.helmOp.EXPECT().Create(gomock.Any()).Do(func(helmOp *fleet.HelmOp) {
		s.Equal("default", helmOp.Namespace)
		s.Equal("autoscaler-default-test-cluster", helmOp.Name)
	}).Return(&fleet.HelmOp{}, nil)
}

func (s *autoscalerSuite) expectHelmOpUpdate(expectedNamespace, expectedName string) {
	s.helmOp.EXPECT().Update(gomock.Any()).Do(func(helmOp *fleet.HelmOp) {
		s.Equal(expectedNamespace, helmOp.Namespace)
		s.Equal(expectedName, helmOp.Name)
	}).Return(&fleet.HelmOp{}, nil)
}

func (s *autoscalerSuite) expectHelmOpDelete(namespace, name string, err error) {
	s.helmOp.EXPECT().Delete(namespace, name, &metav1.DeleteOptions{}).Return(err)
}

// Test cases for ensureFleetHelmOp method

func (s *autoscalerSuite) TestEnsureFleetHelmOp_HappyPath_CreateNewHelmOp() {
	cluster := s.createTestCluster("test-cluster", "default")

	s.expectHelmOpGet("default", "autoscaler-default-test-cluster", nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.expectHelmOpCreate()

	err := s.h.ensureFleetHelmOp(cluster, "v1", 3)
	s.NoError(err)
}

func (s *autoscalerSuite) TestEnsureFleetHelmOp_HappyPath_UpdateExistingHelmOp() {
	cluster := s.createTestCluster("test-cluster", "default")

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

	s.expectHelmOpGet("default", "autoscaler-default-test-cluster", existingHelmOp, nil)
	s.expectHelmOpUpdate("default", "cluster-autoscaler-test-cluster")

	err := s.h.ensureFleetHelmOp(cluster, "v1", 3)
	s.NoError(err)
}

func (s *autoscalerSuite) TestEnsureFleetHelmOp_HappyPath_NoUpdateNeeded() {
	cluster := s.createTestCluster("test-cluster", "default")

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

	s.expectHelmOpGet("default", "autoscaler-default-test-cluster", existingHelmOp, nil)
	s.helmOp.EXPECT().Update(gomock.Any()).Times(0)

	err := s.h.ensureFleetHelmOp(cluster, "v1", 3)
	s.NoError(err)
}

// Test cases for resolveImageTagVersion method

func (s *autoscalerSuite) TestResolveImageTagVersion_HappyPath_KnownVersion() {
	cluster := s.createTestClusterWithControlPlane("test-cluster", "default", "rke.cattle.io", "RKEControlPlane", "test-control-plane")

	s.setupDiscoveryForAPIGroup("rke.cattle.io", "v1", "RKEControlPlane")
	s.setupDynamicClientForRKEControlPlane("v1.34.0+k3s1")

	result := s.h.resolveImageTagVersion(cluster)
	s.Equal("1.34.0-3.4", result)
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
	cluster := s.createTestCluster("test-cluster", "default")

	result := s.h.getChartImageSettings(cluster)
	s.Equal(map[string]any{}, result)
}

func (s *autoscalerSuite) TestGetChartImageSettings_HappyPath_WithValidImage() {
	originalImage := settings.ClusterAutoscalerImage.Get()
	defer func() {
		_ = settings.ClusterAutoscalerImage.Set(originalImage)
	}()
	_ = settings.ClusterAutoscalerImage.Set("registry.example.com/cluster-autoscaler:1.2.3")

	cluster := s.createTestCluster("test-cluster", "default")

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
	cluster := s.createTestClusterWithControlPlane("test-cluster", "default", "rke.cattle.io", "RKEControlPlane", "test-control-plane")

	s.setupDiscoveryForAPIGroup("rke.cattle.io", "v1", "RKEControlPlane")
	s.setupDynamicClientForRKEControlPlane("v1.34.0+k3s1")

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(34, result)
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_HappyPath_CAPIControlPlane() {
	cluster := s.createTestClusterWithControlPlane("test-cluster", "default", "controlplane.cluster.x-k8s.io", "KubeadmControlPlane", "test-control-plane")

	s.setupDiscoveryForAPIGroup("controlplane.cluster.x-k8s.io", "v1beta1", "KubeadmControlPlane")
	s.setupDynamicClientForCAPIControlPlane("v1.33.0+k3s1")

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(33, result)
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_EdgeCase_ErrorGettingControlPlane() {
	cluster := s.createTestClusterWithControlPlane("test-cluster", "default", "rke.cattle.io", "RKEControlPlane", "non-existent")

	s.setupDiscoveryForAPIGroup("rke.cattle.io", "v1", "RKEControlPlane")
	s.dynamicClient.SetGetFunc(func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
		return nil, fmt.Errorf("not found")
	})

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(0, result)
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_EdgeCase_InvalidVersion() {
	cluster := s.createTestClusterWithControlPlane("test-cluster", "default", "rke.cattle.io", "RKEControlPlane", "test-control-plane")

	s.setupDiscoveryForAPIGroup("rke.cattle.io", "v1", "RKEControlPlane")
	s.setupDynamicClientForRKEControlPlane("invalid-version")

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(0, result)
}

func (s *autoscalerSuite) TestGetKubernetesMinorVersion_EdgeCase_NilControlPlaneRef() {
	cluster := s.createTestCluster("test-cluster", "default")

	result := s.h.getKubernetesMinorVersion(cluster)
	s.Equal(0, result)
}

// Test cases for cleanupFleet method

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_HelmOpExists() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, &fleet.HelmOp{}, nil)
	s.expectHelmOpDelete(cluster.Namespace, helmOpName, nil)

	err := s.h.cleanupFleet(cluster)
	s.NoError(err)
}

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_HelmOpDoesNotExist() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, nil, errors.NewNotFound(schema.GroupResource{}, ""))

	err := s.h.cleanupFleet(cluster)
	s.NoError(err)
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_DeleteError() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, &fleet.HelmOp{}, nil)
	s.expectHelmOpDelete(cluster.Namespace, helmOpName, fmt.Errorf("delete failed"))

	err := s.h.cleanupFleet(cluster)
	s.Error(err)
	if err != nil {
		s.Contains(err.Error(), "failed to delete Helm operation")
	}
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_GetError() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, nil, fmt.Errorf("get failed"))

	err := s.h.cleanupFleet(cluster)
	s.Error(err)
	if err != nil {
		s.Contains(err.Error(), "failed to check existence of Helm operation")
	}
}

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_SuccessfulHelmOpDeletion() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, &fleet.HelmOp{}, nil)
	s.expectHelmOpDelete(cluster.Namespace, helmOpName, nil)

	err := s.h.cleanupFleet(cluster)
	s.NoError(err, "Expected no error when successfully cleaning up HelmOp")
}

func (s *autoscalerSuite) TestCleanupFleet_HappyPath_NoHelmOpExists() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, nil, errors.NewNotFound(schema.GroupResource{}, "helmop"))

	err := s.h.cleanupFleet(cluster)
	s.NoError(err, "Expected no error when HelmOp doesn't exist")
}

func (s *autoscalerSuite) TestCleanupFleet_Error_FailedToDeleteHelmOp() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)
	deleteError := fmt.Errorf("failed to delete HelmOp: access denied")

	s.expectHelmOpGet(cluster.Namespace, helmOpName, &fleet.HelmOp{}, nil)
	s.expectHelmOpDelete(cluster.Namespace, helmOpName, deleteError)

	err := s.h.cleanupFleet(cluster)
	s.Error(err, "Expected error when HelmOp deletion fails")
	if err != nil {
		s.Contains(err.Error(), "encountered 1 errors during fleet cleanup")
		s.Contains(err.Error(), "failed to delete Helm operation "+helmOpName+" in namespace "+cluster.Namespace)
		s.Contains(err.Error(), "access denied")
	}
}

func (s *autoscalerSuite) TestCleanupFleet_Error_FailedToCheckHelmOpExistence() {
	cluster := s.createTestCluster("test-cluster", "default")
	helmOpName := helmOpName(cluster)
	checkError := fmt.Errorf("failed to check HelmOp existence: network timeout")

	s.expectHelmOpGet(cluster.Namespace, helmOpName, nil, checkError)

	err := s.h.cleanupFleet(cluster)
	s.Error(err, "Expected error when HelmOp existence check fails")
	if err != nil {
		s.Contains(err.Error(), "encountered 1 errors during fleet cleanup", "Error should mention count of errors")
		s.Contains(err.Error(), "failed to check existence of Helm operation "+helmOpName+" in namespace "+cluster.Namespace, "Error should include HelmOp name and namespace")
		s.Contains(err.Error(), "network timeout", "Original error should be preserved")
	}
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_ClusterWithEmptyName() {
	cluster := s.createTestCluster("", "default")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, nil, errors.NewNotFound(schema.GroupResource{}, "helmop"))

	err := s.h.cleanupFleet(cluster)
	s.NoError(err, "Expected no error when cluster has empty name")
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_ClusterWithEmptyNamespace() {
	cluster := s.createTestCluster("test-cluster", "")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, nil, errors.NewNotFound(schema.GroupResource{}, "helmop"))

	err := s.h.cleanupFleet(cluster)
	s.NoError(err, "Expected no error when cluster has empty namespace")
}

func (s *autoscalerSuite) TestCleanupFleet_EdgeCase_ClusterWithSpecialCharacters() {
	cluster := s.createTestCluster("test-cluster-123", "test-namespace-456")
	helmOpName := helmOpName(cluster)

	s.expectHelmOpGet(cluster.Namespace, helmOpName, &fleet.HelmOp{}, nil)
	s.expectHelmOpDelete(cluster.Namespace, helmOpName, nil)

	err := s.h.cleanupFleet(cluster)
	s.NoError(err, "Expected no error when cluster has special characters in name and namespace")
}
