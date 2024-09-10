package fleetcluster

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/require"
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
		name          string
		cluster       *provv1.Cluster
		status        provv1.ClusterStatus
		clustersCache v3.ClusterCache
		expectedFleet *fleet.Cluster
	}{
		{
			"cluster-has-no-customization",
			cluster,
			clusterStatus,
			newClusterCache(t, map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					apimgmtv3.ClusterSpec{},
				),
			}),
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
			newClusterCache(t, map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					apimgmtv3.ClusterSpec{
						ClusterSpecBase: apimgmtv3.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
								OverrideAffinity:             &linuxAffinity,
								OverrideResourceRequirements: &corev1.ResourceRequirements{},
								AppendTolerations:            []corev1.Toleration{},
							},
						},
					},
				),
			}),
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
			newClusterCache(t, map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					labels,
					apimgmtv3.ClusterSpec{
						ClusterSpecBase: apimgmtv3.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
								OverrideAffinity:             nil,
								OverrideResourceRequirements: resourceReq,
								AppendTolerations:            tolerations,
							},
						},
					},
				),
			}),
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
			h.clustersCache = tt.clustersCache
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
		clustersCache:     newClusterCache(t, nil),
		getPrivateRepoURL: func(*provv1.Cluster, *apimgmtv3.Cluster) string { return "" },
	}

	tests := []struct {
		name          string
		cluster       *provv1.Cluster
		status        provv1.ClusterStatus
		clustersCache v3.ClusterCache
		expectedLen   int
	}{
		{
			"cluster-has-no-cg",
			&provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: provv1.ClusterSpec{},
			},
			provv1.ClusterStatus{
				ClusterName:      "cluster-name",
				ClientSecretName: "client-secret-name",
			},

			newClusterCache(t, map[string]*apimgmtv3.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					map[string]string{
						"cluster-group": "cluster-group-name",
					},
					apimgmtv3.ClusterSpec{Internal: false},
				),
			}),
			1,
		},
		{
			"local-cluster-has-cg-has-label",
			&provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-cluster",
					Namespace: "fleet-local",
				},
				Spec: provv1.ClusterSpec{},
			},
			provv1.ClusterStatus{
				ClusterName:      "local-cluster",
				ClientSecretName: "local-kubeconfig",
			},
			newClusterCache(t, map[string]*apimgmtv3.Cluster{
				"local-cluster": newMgmtCluster(
					"local-cluster",
					map[string]string{
						"cluster-group": "default",
					},
					apimgmtv3.ClusterSpec{Internal: true},
				),
			}),
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.clustersCache = tt.clustersCache
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
		})
	}

}

func newMgmtCluster(name string, labels map[string]string, spec apimgmtv3.ClusterSpec) *apimgmtv3.Cluster {
	spec.DisplayName = name
	mgmtCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: spec,
	}
	apimgmtv3.ClusterConditionReady.SetStatus(mgmtCluster, "True")
	return mgmtCluster

}

// implements v3.ClusterCache
func newClusterCache(t *testing.T, clusters map[string]*apimgmtv3.Cluster) v3.ClusterCache {
	t.Helper()
	ctrl := gomock.NewController(t)
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*apimgmtv3.Cluster, error) {
		if c, ok := clusters[name]; ok {
			return c, nil
		}
		return nil, errNotFound
	}).AnyTimes()
	return clusterCache
}
