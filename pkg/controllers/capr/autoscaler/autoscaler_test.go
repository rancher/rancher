package autoscaler

import (
	"fmt"
	"testing"
	"time"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// mockDynamicGetter is a mock implementation of the DynamicClient interface
type mockDynamicGetter struct {
	mockCtrl *gomock.Controller
	getFunc  func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error)
}

func (m *mockDynamicGetter) Get(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error) {
	if m.getFunc != nil {
		return m.getFunc(gvk, namespace, name)
	}
	return nil, nil
}

func (m *mockDynamicGetter) SetGetFunc(f func(gvk schema.GroupVersionKind, namespace string, name string) (runtime.Object, error)) {
	m.getFunc = f
}

type autoscalerSuite struct {
	suite.Suite

	mockCtrl                   *gomock.Controller
	h                          *autoscalerHandler
	m                          *machineDeploymentReplicaOverrider
	capiClusterCache           *fake.MockCacheInterface[*capi.Cluster]
	capiMachineCache           *fake.MockCacheInterface[*capi.Machine]
	capiMachineDeploymentCache *fake.MockCacheInterface[*capi.MachineDeployment]
	clusterClient              *fake.MockClientInterface[*provv1.Cluster, *provv1.ClusterList]
	clusterCache               *fake.MockCacheInterface[*provv1.Cluster]
	globalRoleClient           *fake.MockNonNamespacedClientInterface[*v3.GlobalRole, *v3.GlobalRoleList]
	globalRoleCache            *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	globalRoleBindingClient    *fake.MockNonNamespacedClientInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList]
	globalRoleBindingCache     *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	userClient                 *fake.MockNonNamespacedClientInterface[*v3.User, *v3.UserList]
	userCache                  *fake.MockNonNamespacedCacheInterface[*v3.User]
	tokenClient                *fake.MockNonNamespacedClientInterface[*v3.Token, *v3.TokenList]
	tokenCache                 *fake.MockNonNamespacedCacheInterface[*v3.Token]
	secretClient               *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]
	secretCache                *fake.MockCacheInterface[*corev1.Secret]
	helmOp                     *fake.MockControllerInterface[*fleet.HelmOp, *fleet.HelmOpList]
	helmOpCache                *fake.MockCacheInterface[*fleet.HelmOp]
	dynamicClient              *mockDynamicGetter
}

func TestAutoscaler(t *testing.T) {
	suite.Run(t, &autoscalerSuite{})
}

func (s *autoscalerSuite) SetupTest() {
	// Create mock controller
	s.mockCtrl = gomock.NewController(s.T())

	// Create mock caches and clients using the correct types from the autoscaler.go file
	s.capiClusterCache = fake.NewMockCacheInterface[*capi.Cluster](s.mockCtrl)
	s.capiMachineCache = fake.NewMockCacheInterface[*capi.Machine](s.mockCtrl)
	s.capiMachineDeploymentCache = fake.NewMockCacheInterface[*capi.MachineDeployment](s.mockCtrl)
	s.clusterClient = fake.NewMockClientInterface[*provv1.Cluster, *provv1.ClusterList](s.mockCtrl)
	s.clusterCache = fake.NewMockCacheInterface[*provv1.Cluster](s.mockCtrl)
	s.globalRoleClient = fake.NewMockNonNamespacedClientInterface[*v3.GlobalRole, *v3.GlobalRoleList](s.mockCtrl)
	s.globalRoleCache = fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](s.mockCtrl)
	s.globalRoleBindingClient = fake.NewMockNonNamespacedClientInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](s.mockCtrl)
	s.globalRoleBindingCache = fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](s.mockCtrl)
	s.userClient = fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](s.mockCtrl)
	s.userCache = fake.NewMockNonNamespacedCacheInterface[*v3.User](s.mockCtrl)
	s.tokenClient = fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](s.mockCtrl)
	s.tokenCache = fake.NewMockNonNamespacedCacheInterface[*v3.Token](s.mockCtrl)
	s.secretClient = fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](s.mockCtrl)
	s.secretCache = fake.NewMockCacheInterface[*corev1.Secret](s.mockCtrl)
	s.helmOp = fake.NewMockControllerInterface[*fleet.HelmOp, *fleet.HelmOpList](s.mockCtrl)
	s.helmOpCache = fake.NewMockCacheInterface[*fleet.HelmOp](s.mockCtrl)
	s.dynamicClient = &mockDynamicGetter{mockCtrl: s.mockCtrl}

	s.h = &autoscalerHandler{
		capiClusterCache:           s.capiClusterCache,
		capiMachineCache:           s.capiMachineCache,
		capiMachineDeploymentCache: s.capiMachineDeploymentCache,
		clusterClient:              s.clusterClient,
		clusterCache:               s.clusterCache,
		globalRoleClient:           s.globalRoleClient,
		globalRoleCache:            s.globalRoleCache,
		globalRoleBindingClient:    s.globalRoleBindingClient,
		globalRoleBindingCache:     s.globalRoleBindingCache,
		userClient:                 s.userClient,
		userCache:                  s.userCache,
		tokenClient:                s.tokenClient,
		tokenCache:                 s.tokenCache,
		secretClient:               s.secretClient,
		secretCache:                s.secretCache,
		helmOp:                     s.helmOp,
		helmOpCache:                s.helmOpCache,
		dynamicClient:              s.dynamicClient,
	}

	s.m = &machineDeploymentReplicaOverrider{
		clusterCache:     s.clusterCache,
		clusterClient:    s.clusterClient,
		capiClusterCache: s.capiClusterCache,
	}
}

func (s *autoscalerSuite) TearDownTest() {
	if s.mockCtrl != nil {
		s.mockCtrl.Finish()
	}
}

// Test cases for isAutoscalingEnabled method

func (s *autoscalerSuite) TestIsAutoscalingEnabled_HappyPath_WithClusterAnnotationAndValidMachineDeployment() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	// Create a machine deployment with min and max annotations
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
				capi.AutoscalerMaxSizeAnnotation: "10",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.True(result, "Expected autoscaling to be enabled")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_HappyPath_WithMultipleMachineDeployments() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	// Create multiple machine deployments, only one with proper annotations
	machineDeployment1 := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md-1",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
				capi.AutoscalerMaxSizeAnnotation: "10",
			},
		},
	}

	machineDeployment2 := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md-2",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "5",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment1, machineDeployment2}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.True(result, "Expected autoscaling to be enabled when at least one machine deployment has proper annotations")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_EmptyMachineDeployments() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	mds := []*capi.MachineDeployment{}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when machine deployments slice is empty")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_ClusterWithoutAutoscalingAnnotation() {
	// Create a test cluster without autoscaling annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Create a machine deployment with min and max annotations
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
				capi.AutoscalerMaxSizeAnnotation: "10",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when cluster doesn't have autoscaling annotation")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_MachineDeploymentsMissingMinAnnotation() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	// Create a machine deployment missing min annotation
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMaxSizeAnnotation: "10",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when machine deployment is missing min annotation")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_MachineDeploymentsMissingMaxAnnotation() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	// Create a machine deployment missing max annotation
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when machine deployment is missing max annotation")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_ClusterWithAutoscalingAnnotationSetToFalse() {
	// Create a test cluster with autoscaling annotation set to false
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "false"},
		},
	}

	// Create a machine deployment with min and max annotations
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
				capi.AutoscalerMaxSizeAnnotation: "10",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when cluster annotation is set to false")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_ClusterWithInvalidAutoscalingAnnotation() {
	// Create a test cluster with invalid autoscaling annotation value
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "invalid"},
		},
	}

	// Create a machine deployment with min and max annotations
	machineDeployment := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
				capi.AutoscalerMaxSizeAnnotation: "10",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when cluster annotation has invalid value")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_NilMachineDeployments() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	// Call the method with nil machine deployments
	result := s.h.isAutoscalingEnabled(cluster, nil)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when machine deployments is nil")
}

func (s *autoscalerSuite) TestIsAutoscalingEnabled_EdgeCase_AllMachineDeploymentsMissingAnnotations() {
	// Create a test cluster with autoscaling enabled annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerEnabledAnnotation: "true"},
		},
	}

	// Create machine deployments without proper annotations
	machineDeployment1 := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md-1",
			Namespace: "default",
		},
	}

	machineDeployment2 := &capi.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-md-2",
			Namespace: "default",
			Annotations: map[string]string{
				capi.AutoscalerMinSizeAnnotation: "2",
			},
		},
	}

	mds := []*capi.MachineDeployment{machineDeployment1, machineDeployment2}

	// Call the method
	result := s.h.isAutoscalingEnabled(cluster, mds)

	// Assert the result
	s.False(result, "Expected autoscaling to be disabled when no machine deployments have proper annotations")
}

func (s *autoscalerSuite) TestAutoscalingPaused_HappyPath_PausedSetToTrue() {
	// Create a test cluster with autoscaling paused annotation set to "true"
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerPausedAnnotation: "true"},
		},
	}

	// Call the function
	result := autoscalingPaused(cluster)

	// Assert the result
	s.True(result, "Expected autoscaling to be paused when annotation is set to 'true'")
}

func (s *autoscalerSuite) TestAutoscalingPaused_HappyPath_PausedSetToFalse() {
	// Create a test cluster with autoscaling paused annotation set to "false"
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{capr.ClusterAutoscalerPausedAnnotation: "false"},
		},
	}

	// Call the function
	result := autoscalingPaused(cluster)

	// Assert the result
	s.False(result, "Expected autoscaling not to be paused when annotation is set to 'false'")
}

func (s *autoscalerSuite) TestAutoscalingPaused_EdgeCase_NoPauseAnnotation() {
	// Create a test cluster without pause annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Call the function
	result := autoscalingPaused(cluster)

	// Assert the result - should return false when annotation is not present
	s.False(result, "Expected autoscaling not to be paused when pause annotation is missing")
}

func (s *autoscalerSuite) TestAutoscalingPaused_EdgeCase_EmptyAnnotationsMap() {
	// Create a test cluster with empty annotations map
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
	}

	// Call the function
	result := autoscalingPaused(cluster)

	// Assert the result - should return false when annotations map is empty
	s.False(result, "Expected autoscaling not to be paused when annotations map is empty")
}

func (s *autoscalerSuite) TestAutoscalingPaused_EdgeCase_PausedAnnotationWithDifferentValues() {
	testCases := []struct {
		name            string
		annotationValue string
		expectedResult  bool
	}{
		{"Paused set to 'yes'", "yes", false},
		{"Paused set to '1'", "1", false},
		{"Paused set to empty string", "", false},
		{"Paused set to 'TRUE' (uppercase)", "TRUE", false},
		{"Paused set to 'True' (mixed case)", "True", false},
		{"Paused set to 'tRuE' (mixed case)", "tRuE", false},
		{"Paused set to 'on'", "on", false},
		{"Paused set to 'enabled'", "enabled", false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Create a test cluster with different annotation values
			cluster := &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "default",
					Annotations: map[string]string{capr.ClusterAutoscalerPausedAnnotation: tc.annotationValue},
				},
			}

			// Call the function
			result := autoscalingPaused(cluster)

			// Assert the result
			s.Equal(tc.expectedResult, result, "Expected autoscaling paused result to match expected value for annotation '%s'", tc.annotationValue)
		})
	}
}

func (s *autoscalerSuite) TestAutoscalingPaused_Integration_ClusterAutoscalerPausedAnnotationConstant() {
	// Test that the function correctly uses the capr.ClusterAutoscalerPausedAnnotation constant
	// This verifies integration with the external dependency

	// Create a test cluster with the exact annotation key from the constant
	expectedAnnotationKey := capr.ClusterAutoscalerPausedAnnotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: map[string]string{expectedAnnotationKey: "true"},
		},
	}

	// Call the function
	result := autoscalingPaused(cluster)

	// Assert the result - should return true when using the correct annotation key
	s.True(result, "Expected autoscaling to be paused when using the correct annotation constant")

	// Verify that the constant has the expected value
	expectedValue := "provisioning.cattle.io/cluster-autoscaler-paused"
	s.Equal(expectedValue, capr.ClusterAutoscalerPausedAnnotation, "ClusterAutoscalerPausedAnnotation constant should have expected value")
}

func (s *autoscalerSuite) TestAutoscalingPaused_EdgeCase_ClusterWithMultipleAnnotations() {
	// Create a test cluster with multiple annotations including the pause annotation
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			Annotations: map[string]string{
				capr.ClusterAutoscalerPausedAnnotation:  "true",
				"other.annotation/1":                    "value1",
				"other.annotation/2":                    "value2",
				capr.ClusterAutoscalerEnabledAnnotation: "false",
			},
		},
	}

	// Call the function
	result := autoscalingPaused(cluster)

	// Assert the result - should return true despite other annotations being present
	s.True(result, "Expected autoscaling to be paused even when other annotations are present")
}

// Test cases for pauseAutoscaling method

func (s *autoscalerSuite) TestPauseAutoscaling_HappyPath_SuccessfulScaling() {
	// Arrange - Create test cluster and secret
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kubeconfig-test-cluster",
			ResourceVersion: "12345",
		},
	}

	// Set up mock expectations
	s.secretCache.EXPECT().Get(cluster.Namespace, kubeconfigSecretName(cluster)).Return(secret, nil)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName(cluster)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOp.EXPECT().Create(gomock.Any()).DoAndReturn(func(helmOp *fleet.HelmOp) (*fleet.HelmOp, error) {
		s.Equal(0, helmOp.Spec.BundleDeploymentOptions.Helm.Values.Data["replicaCount"], "HelmOp should be scaled to 0 replicas")
		return &fleet.HelmOp{}, nil
	})

	// Act - Call the method
	err := s.h.pauseAutoscaling(cluster)

	// Assert - Verify results
	s.NoError(err, "Expected no error when successfully pausing autoscaling")
}

func (s *autoscalerSuite) TestPauseAutoscaling_Error_SecretNotFound() {
	// Arrange - Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
	}

	// Set up mock expectations to return not found error
	s.secretCache.EXPECT().Get(cluster.Namespace, kubeconfigSecretName(cluster)).Return(nil, errors.NewNotFound(corev1.Resource("secrets"), "secret"))

	// Act - Call the method
	err := s.h.pauseAutoscaling(cluster)

	// Assert - Verify error
	s.Error(err, "Expected error when kubeconfig secret is not found")
	s.Contains(err.Error(), "not found", "Error should indicate that secret was not found")
}

func (s *autoscalerSuite) TestPauseAutoscaling_Error_FailedToScaleHelmOp() {
	// Arrange - Create test cluster and secret
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kubeconfig-test-cluster",
			ResourceVersion: "12345",
		},
	}

	scaleError := fmt.Errorf("failed to scale HelmOp")

	// Set up mock expectations
	s.secretCache.EXPECT().Get(cluster.Namespace, kubeconfigSecretName(cluster)).Return(secret, nil)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName(cluster)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOp.EXPECT().Create(gomock.Any()).DoAndReturn(func(helmOp *fleet.HelmOp) (*fleet.HelmOp, error) {
		s.Equal(0, helmOp.Spec.BundleDeploymentOptions.Helm.Values.Data["replicaCount"], "HelmOp should be scaled to 0 replicas")
		return nil, scaleError
	})

	// Act - Call the method
	err := s.h.pauseAutoscaling(cluster)

	// Assert - Verify error
	s.Error(err, "Expected error when HelmOp scaling fails")
	s.Equal(scaleError, err, "Error should be the same as the HelmOp scaling error")
}

// Test cases for ensureCleanup method

func (s *autoscalerSuite) TestEnsureCleanup_HappyPath_NilCluster() {
	// Arrange - Call with nil cluster
	result, err := s.h.ensureCleanup("test-key", nil)

	// Assert - Verify early return when cluster is nil
	s.Nil(result, "Expected nil result when cluster is nil")
	s.NoError(err, "Expected no error when cluster is nil")
}

func (s *autoscalerSuite) TestEnsureCleanup_HappyPath_ClusterWithDeletionTimestamp() {
	// Arrange - Create cluster with DeletionTimestamp set
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "default",
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify early return when DeletionTimestamp is set
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when cluster has DeletionTimestamp")
}

func (s *autoscalerSuite) TestEnsureCleanup_HappyPath_SuccessfulCleanup() {
	// Arrange - Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Set up mock expectations for successful cleanupRBAC and cleanupFleet
	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)
	helmOpName := helmOpName(cluster)

	// User doesn't exist (should not cause error)
	s.userCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Global role doesn't exist (should not cause error)
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Global role binding doesn't exist (should not cause error)
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Token doesn't exist (should not cause error)
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Secret doesn't exist (should not cause error)
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// HelmOp doesn't exist (should not cause error)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify successful cleanup
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when cleanup is successful")
}

func (s *autoscalerSuite) TestEnsureCleanup_Error_HandleUninstallFails() {
	// Arrange - Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)
	helmOpName := helmOpName(cluster)

	// User doesn't exist
	s.userCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Global role doesn't exist
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Global role binding doesn't exist
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Token doesn't exist
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Secret exists but deletion fails
	secretDeleteError := fmt.Errorf("failed to delete secret: access denied")
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(&corev1.Secret{}, nil)
	s.secretClient.EXPECT().Delete(cluster.Namespace, secretName, gomock.Any()).Return(secretDeleteError)

	// HelmOp doesn't exist (should not cause error)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify error propagation
	s.Nil(result, "Expected nil result when cleanup fails")
	s.Error(err, "Expected error when cleanup fails")
	s.Contains(err.Error(), "failed to delete secret "+secretName+" in namespace "+cluster.Namespace, "Error should include secret name and namespace")
	s.Contains(err.Error(), "access denied", "Original error should be preserved")
}

func (s *autoscalerSuite) TestEnsureCleanup_EdgeCase_ClusterWithEmptyName() {
	// Arrange - Create cluster with empty name
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "",
			Namespace: "default",
		},
	}

	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)

	// All resources don't exist (should handle empty names gracefully)
	s.userCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName(cluster)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify successful cleanup with empty name
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when cluster has empty name")
}

func (s *autoscalerSuite) TestEnsureCleanup_EdgeCase_ClusterWithEmptyNamespace() {
	// Arrange - Create cluster with empty namespace
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "",
		},
	}

	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)

	// All resources don't exist (should handle empty namespace gracefully)
	s.userCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName(cluster)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify successful cleanup with empty namespace
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when cluster has empty namespace")
}

func (s *autoscalerSuite) TestEnsureCleanup_EdgeCase_ClusterWithSpecialCharacters() {
	// Arrange - Create cluster with special characters
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-123!@#",
			Namespace: "test-namespace-456$%^",
		},
	}

	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)

	// All resources don't exist (should handle special characters gracefully)
	s.userCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName(cluster)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify successful cleanup with special characters
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when cluster has special characters")
}

func (s *autoscalerSuite) TestEnsureCleanup_EdgeCase_ClusterWithOldDeletionTimestamp() {
	// Arrange - Create cluster with DeletionTimestamp set to past time
	pastTime := time.Now().Add(-1 * time.Hour)
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "default",
			DeletionTimestamp: &metav1.Time{Time: pastTime},
		},
	}

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify early return regardless of DeletionTimestamp value
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when cluster has old DeletionTimestamp")
}

func (s *autoscalerSuite) TestEnsureCleanup_Error_MultipleCleanupFailures() {
	// Arrange - Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)
	helmOpName := helmOpName(cluster)

	// User exists but deletion fails
	userDeleteError := fmt.Errorf("failed to delete user: access denied")
	s.userCache.EXPECT().Get(userName).Return(&v3.User{}, nil)
	s.userClient.EXPECT().Delete(userName, gomock.Any()).Return(userDeleteError)

	// Global role doesn't exist (should continue despite user deletion failure)
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Global role binding doesn't exist
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Token doesn't exist
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Secret exists but deletion fails
	secretDeleteError := fmt.Errorf("failed to delete secret: not found")
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(&corev1.Secret{}, nil)
	s.secretClient.EXPECT().Delete(cluster.Namespace, secretName, gomock.Any()).Return(secretDeleteError)

	// HelmOp doesn't exist (should not cause error)
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method
	result, err := s.h.ensureCleanup("test-key", cluster)

	// Assert - Verify multiple error propagation
	s.Nil(result, "Expected nil result when multiple cleanups fail")
	s.Error(err, "Expected error when multiple cleanups fail")
	s.Contains(err.Error(), "encountered 2 errors during cleanup", "Error should mention count of errors")
	s.Contains(err.Error(), "failed to delete user "+userName, "Error should include user name")
	s.Contains(err.Error(), "access denied", "First original error should be preserved")
	s.Contains(err.Error(), "failed to delete secret "+secretName+" in namespace "+cluster.Namespace, "Error should include secret name and namespace")
	s.Contains(err.Error(), "not found", "Second original error should be preserved")
}

func (s *autoscalerSuite) TestEnsureCleanup_EdgeCase_EmptyKeyParameter() {
	// Arrange - Create test cluster
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	userName := autoscalerUserName(cluster)
	globalRoleName := globalRoleName(cluster)
	globalRoleBindingName := globalRoleBindingName(cluster)
	secretName := kubeconfigSecretName(cluster)

	// All resources don't exist (should handle empty key gracefully)
	s.userCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleCache.EXPECT().Get(globalRoleName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.tokenCache.EXPECT().Get(userName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.secretCache.EXPECT().Get(cluster.Namespace, secretName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
	s.helmOpCache.EXPECT().Get(cluster.Namespace, helmOpName(cluster)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

	// Call the method with empty key
	result, err := s.h.ensureCleanup("", cluster)

	// Assert - Verify successful cleanup regardless of key value
	s.Equal(cluster, result, "Expected same cluster object returned")
	s.NoError(err, "Expected no error when key parameter is empty")
}

// Test cases for syncHelmOpStatus method
