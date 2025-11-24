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
	errNotFound = fmt.Errorf("not found")

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
	// used in tests to assert ordering with non-nil TolerationSeconds
	tolerationSeconds30 int64 = 30
	tolerationSeconds10 int64 = 10
	tolerations               = []corev1.Toleration{
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

func TestAssignWorkspace(t *testing.T) {
	require := require.New(t)

	h := &handler{}

	tests := []struct {
		name    string
		cluster *apimgmtv3.Cluster
	}{
		{
			name: "does not modify cluster without fleet workspace",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "fleet-default",
					Annotations: map[string]string{
						externallyManagedAnn: "true",
					},
				},
				Spec: apimgmtv3.ClusterSpec{},
			},
		},
		{
			name: "does not modify cluster with fleet workspace set",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-cluster",
					Namespace: "fleet-local",
					Annotations: map[string]string{
						externallyManagedAnn: "true",
					},
				},
				Spec: apimgmtv3.ClusterSpec{
					FleetWorkspaceName: "specific-name",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := h.assignWorkspace("", tt.cluster)

			if err != nil {
				t.Errorf("Expected nil err")
			}

			if cluster == nil {
				t.Errorf("Expected non-nil cluster: %v", err)
			}

			require.Equal(cluster, tt.cluster)
		})
	}
}

func TestCreateCluster(t *testing.T) {
	h := &handler{
		getPrivateRepoURL: func(*provv1.Cluster, *apimgmtv3.Cluster) string { return "" },
	}

	tests := []struct {
		name                 string
		cluster              *provv1.Cluster
		status               provv1.ClusterStatus
		cachedClusters       map[string]*apimgmtv3.Cluster
		nodes                []corev1.Node
		cpTaintsLabel        string
		expectedLen          int
		expectedLabels       map[string]string
		expectedAnnotations  map[string]string
		expectedTolerations  []corev1.Toleration
		expectedErrorMessage string
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
			nodes: []corev1.Node{
				{
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "key_taint1",
								Value:  "value_taint1",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "key_taint2",
								Value:  "value_taint2",
								Effect: corev1.TaintEffectPreferNoSchedule,
							},
						},
					},
				},
			},
			cpTaintsLabel: "node-role.kubernetes.io/control-plane=true",
			// external cluster have no CP taints added to tolerations
			expectedTolerations: []corev1.Toleration{},
			expectedLen:         1, // cluster only
			expectedLabels: map[string]string{
				"management.cattle.io/cluster-name":         "cluster-name",
				"management.cattle.io/cluster-display-name": "cluster-name",
				"cluster-group": "cluster-group-name",
			},
		},
		{
			name: "creates only cluster when external extra tolerations",
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
						ClusterSpecBase: apimgmtv3.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
								AppendTolerations: []corev1.Toleration{
									{
										Key:      "key1",
										Value:    "value1",
										Effect:   corev1.TaintEffectPreferNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "key2",
										Value:    "value2",
										Effect:   corev1.TaintEffectNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
								},
							},
						},
					},
				),
			},
			nodes: []corev1.Node{
				{
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "key_taint1",
								Value:  "value_taint1",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "key_taint2",
								Value:  "value_taint2",
								Effect: corev1.TaintEffectPreferNoSchedule,
							},
						},
					},
				},
			},
			cpTaintsLabel: "node-role.kubernetes.io/control-plane=true",
			// external cluster have no CP taints added to tolerations
			expectedTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key2",
					Value:    "value2",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
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
			cpTaintsLabel: "node-role.kubernetes.io/control-plane=true",
			expectedLen:   2, // cluster and cluster group
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
		{
			name: "creates internal cluster and cluster group with cp tolerations, cp label 1",
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
						ClusterSpecBase: apimgmtv3.ClusterSpecBase{
							FleetAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
								AppendTolerations: []corev1.Toleration{
									// intentionally added in bad (unsorted) order to validate sorting
									{
										Key:      "extraKey2",
										Value:    "extraValue2",
										Effect:   corev1.TaintEffectPreferNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "akey",
										Value:    "valueB",
										Effect:   corev1.TaintEffectPreferNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "extraKey1",
										Value:    "extraValue1",
										Effect:   corev1.TaintEffectPreferNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "akey",
										Value:    "valueA",
										Effect:   corev1.TaintEffectPreferNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "akey",
										Value:    "valueA",
										Effect:   corev1.TaintEffectNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "existsKey",
										Operator: corev1.TolerationOpExists,
									},
									{
										Key:               "bkey",
										Value:             "value",
										Effect:            corev1.TaintEffectNoSchedule,
										Operator:          corev1.TolerationOpEqual,
										TolerationSeconds: &tolerationSeconds30,
									},
									{
										Key:      "bkey",
										Value:    "value",
										Effect:   corev1.TaintEffectNoSchedule,
										Operator: corev1.TolerationOpEqual,
										// nil TolerationSeconds - should sort before non-nil
									},
									{
										Key:               "bkey",
										Value:             "value",
										Effect:            corev1.TaintEffectNoSchedule,
										Operator:          corev1.TolerationOpEqual,
										TolerationSeconds: &tolerationSeconds10,
									},
									{
										Key:      "bkey",
										Value:    "value2",
										Effect:   corev1.TaintEffectNoSchedule,
										Operator: corev1.TolerationOpEqual,
									},
									{
										Key:      "zkey",
										Operator: corev1.TolerationOpExists,
									},
								},
							},
						},
					},
				),
			},
			nodes: []corev1.Node{
				{
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "key_taint1",
								Value:  "value_taint1",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "key_taint2",
								Value:  "value_taint2",
								Effect: corev1.TaintEffectPreferNoSchedule,
							},
						},
					},
				},
			},
			cpTaintsLabel: "node-role.kubernetes.io/control-plane=true",
			expectedTolerations: []corev1.Toleration{
				{
					Key:      "akey",
					Value:    "valueA",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "akey",
					Value:    "valueA",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "akey",
					Value:    "valueB",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "bkey",
					Value:    "value",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
					// nil TolerationSeconds sorts before non-nil
				},
				{
					Key:               "bkey",
					Value:             "value",
					Effect:            corev1.TaintEffectNoSchedule,
					Operator:          corev1.TolerationOpEqual,
					TolerationSeconds: &tolerationSeconds10,
				},
				{
					Key:               "bkey",
					Value:             "value",
					Effect:            corev1.TaintEffectNoSchedule,
					Operator:          corev1.TolerationOpEqual,
					TolerationSeconds: &tolerationSeconds30,
				},
				{
					Key:      "bkey",
					Value:    "value2",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "existsKey",
					Operator: corev1.TolerationOpExists,
				},
				{
					Key:      "extraKey1",
					Value:    "extraValue1",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "extraKey2",
					Value:    "extraValue2",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key_taint1",
					Value:    "value_taint1",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key_taint2",
					Value:    "value_taint2",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "zkey",
					Operator: corev1.TolerationOpExists,
				},
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
			name: "creates internal cluster and cluster group with cp tolerations, cp label 2",
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
			nodes: []corev1.Node{
				{
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "key_taint1",
								Value:  "value_taint1",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "key_taint2",
								Value:  "value_taint2_2",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "key_taint2",
								Value:  "value_taint2_1",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "key_taint2",
								Value:  "value_taint2_2",
								Effect: corev1.TaintEffectPreferNoSchedule,
							},
						},
					},
				},
			},
			cpTaintsLabel: "node-role.kubernetes.io/controlplane=true",
			expectedTolerations: []corev1.Toleration{
				{
					Key:      "key_taint1",
					Value:    "value_taint1",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key_taint2",
					Value:    "value_taint2_1",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key_taint2",
					Value:    "value_taint2_2",
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key_taint2",
					Value:    "value_taint2_2",
					Effect:   corev1.TaintEffectPreferNoSchedule,
					Operator: corev1.TolerationOpEqual,
				},
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
			name: "error when calling nodes List",
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
			cpTaintsLabel:        "return-an-error",
			expectedErrorMessage: "failed to list control plane nodes: node list error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			h.clustersCache = newClusterCache(t, ctrl, tt.cachedClusters)
			h.nodesController = newFakeNodesController(t, ctrl, tt.nodes, tt.cpTaintsLabel)

			objs, _, err := h.createCluster(tt.cluster, tt.status)

			if tt.expectedErrorMessage != "" {
				require.NotNil(t, err)
				require.Equal(t, err.Error(), tt.expectedErrorMessage)
			} else {
				require.Nil(t, err)

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

					if len(tt.expectedTolerations) == 0 {
						require.Empty(t, cluster.Spec.AgentTolerations)
					} else {
						require.Equal(t, tt.expectedTolerations, cluster.Spec.AgentTolerations)
					}
				}

				if !foundCluster {
					t.Errorf("Did not find expected cluster %v among created objects %v", tt.cluster, objs)
				}
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

// implements corecontrollers.NodeController List
func newFakeNodesController(t *testing.T, ctrl *gomock.Controller, nodes []corev1.Node, labelSelector string) corecontrollers.NodeController {
	t.Helper()
	nodesController := fake.NewMockNonNamespacedControllerInterface[*corev1.Node, *corev1.NodeList](ctrl)
	nodesController.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*corev1.NodeList, error) {
		switch labelSelector {
		case opts.LabelSelector:
			return &corev1.NodeList{Items: nodes}, nil
		case "return-an-error":
			return nil, fmt.Errorf("node list error")
		default:
			return &corev1.NodeList{}, nil
		}
	}).AnyTimes()
	return nodesController
}
