// mockgen --build_flags=--mod=mod -package fleetcluster -destination=./mock_cluster_host_getter_test.go github.com/rancher/rancher/pkg/controllers/provisioningv2/fleetcluster ClusterHostGetter
package fleetcluster

import (
	"fmt"
	"testing"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errNotImplemented = fmt.Errorf("unimplemented")
	errNotFound       = fmt.Errorf("not found")

	builtinAffinity = corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "fleet.cattle.io/agent",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		},
	}
	linuxAffinity = corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/os",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"linux"},
							},
						},
					},
				},
			},
		},
	}
	resourceReq = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	tolerations = []corev1.Toleration{
		{
			Key:      "key",
			Operator: corev1.TolerationOpEqual,
			Value:    "value",
		},
	}
)

func TestClusterCustomization(t *testing.T) {
	require := require.New(t)

	h := &handler{
		getPrivateRepoURL: func(*provv1.Cluster, *apimgmtv3.Cluster) string { return "" },
	}

	cluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster", Namespace: "test-namespace",
		},
		Spec: provv1.ClusterSpec{},
	}
	clusterStatus := provv1.ClusterStatus{ClusterName: "cluster-name", ClientSecretName: "client-secret-name"}

	labels := map[string]string{"cluster-group": "cluster-group-name"}

	tests := []struct {
		name           string
		cluster        *provv1.Cluster
		status         provv1.ClusterStatus
		cachedClusters map[string]*apimgmtv3.Cluster
		expectedFleet  *fleet.Cluster
	}{
		{
			"cluster-has-no-customization",
			cluster,
			clusterStatus,
			map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					nil,
					apimgmtv3.ClusterSpec{
						FleetWorkspaceName: "fleet-default",
					},
				),
			},
			&fleet.Cluster{
				Spec: fleet.ClusterSpec{
					AgentAffinity: &builtinAffinity,
				},
			},
		},
		{
			"cluster-has-affinity-override",
			cluster,
			clusterStatus,
			map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					nil,
					apimgmtv3.ClusterSpec{
						FleetWorkspaceName: "fleet-default",
						ClusterSpecBase: apimgmtv3.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
								OverrideAffinity:             &linuxAffinity,
								OverrideResourceRequirements: &corev1.ResourceRequirements{},
								AppendTolerations:            []corev1.Toleration{},
							},
						},
					},
				),
			},
			&fleet.Cluster{
				Spec: fleet.ClusterSpec{
					AgentAffinity:    &linuxAffinity,
					AgentResources:   &corev1.ResourceRequirements{},
					AgentTolerations: []corev1.Toleration{},
				},
			},
		},
		{
			"cluster-has-custom-tolerations-and-resources",
			cluster,
			clusterStatus,
			map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					nil,
					apimgmtv3.ClusterSpec{
						FleetWorkspaceName: "fleet-default",
						ClusterSpecBase: apimgmtv3.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
								OverrideAffinity:             nil,
								OverrideResourceRequirements: resourceReq,
								AppendTolerations:            tolerations,
							},
						},
					},
				),
			},
			&fleet.Cluster{
				Spec: fleet.ClusterSpec{
					AgentAffinity:    &builtinAffinity,
					AgentResources:   resourceReq,
					AgentTolerations: tolerations,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			h.clustersCache = newClusterCache(t, ctrl, tt.cachedClusters)
			objs, _, err := h.createCluster(tt.cluster, tt.status)
			require.Nil(err)
			require.NotNil(objs)

			require.Equal(1, len(objs))

			fleetCluster, ok := objs[0].(*fleet.Cluster)
			if !ok {
				t.Errorf("Expected fleet cluster, got %t", objs[0])
			}

			require.Equal(tt.expectedFleet.Spec.AgentAffinity, fleetCluster.Spec.AgentAffinity)
			require.Equal(tt.expectedFleet.Spec.AgentResources, fleetCluster.Spec.AgentResources)
			require.Equal(tt.expectedFleet.Spec.AgentTolerations, fleetCluster.Spec.AgentTolerations)
		})
	}
}

func TestCreateCluster(t *testing.T) {
	h := &handler{
		getPrivateRepoURL: func(*provv1.Cluster, *apimgmtv3.Cluster) string { return "" },
	}

	tests := []struct {
		name                string
		cluster             *provv1.Cluster
		status              provv1.ClusterStatus
		cachedClusters      map[string]*apimgmtv3.Cluster
		expectedLen         int
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name: "creates only cluster when external",
			cluster: &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
				},
				Spec: provv1.ClusterSpec{},
			},
			status: provv1.ClusterStatus{
				ClusterName:      "cluster-name",
				ClientSecretName: "client-secret-name",
			},

			cachedClusters: map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					map[string]string{
						"cluster-group": "cluster-group-name",
					},
					nil,
					apimgmtv3.ClusterSpec{
						FleetWorkspaceName: "fleet-default",
						Internal:           false,
					},
				),
			},
			expectedLen: 1, // cluster only
			expectedLabels: map[string]string{
				"management.cattle.io/cluster-name":         "cluster-name",
				"management.cattle.io/cluster-display-name": "cluster-name",
				"cluster-group": "cluster-group-name",
			},
		},
		{
			name: "creates internal cluster and cluster group",
			cluster: &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-cluster",
					Namespace: "fleet-local",
				},
				Spec: provv1.ClusterSpec{},
			},
			status: provv1.ClusterStatus{
				ClusterName:      "local-cluster",
				ClientSecretName: "local-kubeconfig",
			},
			cachedClusters: map[string]*apimgmtv3.Cluster{
				"local-cluster": newMgmtCluster(
					"local-cluster",
					map[string]string{
						"cluster-group": "cluster-group-name",
					},
					nil,
					apimgmtv3.ClusterSpec{
						FleetWorkspaceName: "fleet-local",
						Internal:           true,
					},
				),
			},
			expectedLen: 2, // cluster and cluster group
			expectedLabels: map[string]string{
				"management.cattle.io/cluster-name":         "local-cluster",
				"management.cattle.io/cluster-display-name": "local-cluster",
				"name":          "local",
				"cluster-group": "cluster-group-name",
			},
		},
		{
			name: "creates cluster with filtered labels and annotations from management cluster",
			cluster: &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
					Labels: map[string]string{
						"foo-label":                 "bar",
						"kubectl.kubernetes.io/foo": "foovalue",
						"blah.cattle.io/meh":        "bleh",
					},
					Annotations: map[string]string{
						"foo-annotation":            "bar",
						"kubectl.kubernetes.io/foo": "foovalue",
						"blah.cattle.io/meh":        "bleh",
					},
				},
				Spec: provv1.ClusterSpec{},
			},
			status: provv1.ClusterStatus{
				ClusterName:      "cluster-name",
				ClientSecretName: "client-secret-name",
			},

			cachedClusters: map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					map[string]string{
						"cluster-group":             "cluster-group-name",
						"kubectl.kubernetes.io/foo": "foovalue", // should be filtered out
						"blah.cattle.io/meh":        "bleh",     // should be filtered out
						"foo-label":                 "bar",
					},
					map[string]string{
						"test-annotation-key":       "test-value",
						"kubectl.kubernetes.io/foo": "foovalue", // should be filtered out
						"blah.cattle.io/meh":        "bleh",     // should be filtered out
						"foo-annotation":            "bar",
					},
					apimgmtv3.ClusterSpec{
						FleetWorkspaceName: "fleet-default",
						Internal:           false,
					},
				),
			},
			expectedLen: 1, // cluster only
			expectedLabels: map[string]string{
				"management.cattle.io/cluster-name":         "cluster-name",
				"management.cattle.io/cluster-display-name": "cluster-name",
				"cluster-group": "cluster-group-name",
				"foo-label":     "bar",
			},
			expectedAnnotations: map[string]string{
				"foo-annotation":      "bar",
				"test-annotation-key": "test-value",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			h.clustersCache = newClusterCache(t, ctrl, tt.cachedClusters)

			objs, _, err := h.createCluster(tt.cluster, tt.status)

			if objs == nil {
				t.Errorf("Expected non-nil objs: %v", err)
			}

			if err != nil {
				t.Errorf("Expected nil err")
			}

			if len(objs) != tt.expectedLen {
				t.Errorf("Expected %d objects, got %d", tt.expectedLen, len(objs))
			}

			foundCluster := false
			for _, obj := range objs {
				cluster, ok := obj.(*fleet.Cluster)

				if !ok {
					continue
				}

				if cluster.Name != tt.cluster.Name || cluster.Namespace != tt.cluster.Namespace {
					continue
				}

				foundCluster = true

				require.Equal(t, tt.expectedLabels, cluster.Labels)

				if len(tt.expectedAnnotations) == 0 {
					require.Empty(t, cluster.Annotations)
				} else {
					require.Equal(t, tt.expectedAnnotations, cluster.Annotations)
				}
			}

			if !foundCluster {
				t.Errorf("Did not find expected cluster %v among created objects %v", tt.cluster, objs)
			}

		})
	}
}

func newMgmtCluster(
	name string,
	labels map[string]string,
	annotations map[string]string,
	spec apimgmtv3.ClusterSpec,
) *apimgmtv3.Cluster {
	spec.DisplayName = name
	mgmtCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: spec,
	}
	apimgmtv3.ClusterConditionReady.SetStatus(mgmtCluster, "True")
	return mgmtCluster

}

// implements v3.ClusterCache
func newClusterCache(t *testing.T, ctrl *gomock.Controller, clusters map[string]*apimgmtv3.Cluster) v3.ClusterCache {
	t.Helper()
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*apimgmtv3.Cluster, error) {
		if c, ok := clusters[name]; ok {
			return c, nil
		}
		return nil, errNotFound
	}).AnyTimes()
	return clusterCache
}

// newSecretsCache returns a mock secrets cache.
func newSecretsCache(t *testing.T, ctrl *gomock.Controller, namespace string, secrets map[string]*corev1.Secret) *fake.MockCacheInterface[*corev1.Secret] {
	t.Helper()
	secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretCache.EXPECT().Get(namespace, "local-kubeconfig").
		Return(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local-kubeconfig",
				},
				Data: map[string][]byte{},
			}, nil).
		AnyTimes()
	return secretCache
}

// newSecretsController returns a mock secrets controller.
func newSecretsController(
	t *testing.T,
	namespace string,
	updatedSecret *corev1.Secret,
) corecontrollers.SecretController {
	t.Helper()
	ctrl := gomock.NewController(t)

	return fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
}
