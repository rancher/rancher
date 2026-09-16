package machineroletaint

import (
	"fmt"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestTaintsNeedUpdate(t *testing.T) {
	h := &handler{}

	tests := []struct {
		name                  string
		nodeTaints            []corev1.Taint
		expectedTaints        []corev1.Taint
		expectUpdate          bool
		expectedToAddCount    int
		expectedToRemoveCount int
	}{
		{
			name:                  "no taints - no update needed",
			nodeTaints:            nil,
			expectedTaints:        nil,
			expectUpdate:          false,
			expectedToAddCount:    0,
			expectedToRemoveCount: 0,
		},
		{
			name:       "add control-plane taint",
			nodeTaints: nil,
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:          true,
			expectedToAddCount:    1,
			expectedToRemoveCount: 0,
		},
		{
			name: "remove control-plane taint",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints:        nil,
			expectUpdate:          true,
			expectedToAddCount:    0,
			expectedToRemoveCount: 1,
		},
		{
			name: "preserve user taints",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				{Key: "custom-taint", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints:        nil,
			expectUpdate:          true,
			expectedToAddCount:    0,
			expectedToRemoveCount: 1, // Only remove control-plane, preserve custom
		},
		{
			name: "taints match - no update",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:          false,
			expectedToAddCount:    0,
			expectedToRemoveCount: 0,
		},
		{
			name: "swap taints - remove etcd, add control-plane",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/etcd", Effect: corev1.TaintEffectNoExecute},
			},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:          true,
			expectedToAddCount:    1,
			expectedToRemoveCount: 1,
		},
		{
			name: "reconcile drifted value - remove stale, re-add expected",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Value: "tampered", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			expectUpdate:          true,
			expectedToAddCount:    1,
			expectedToRemoveCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toAdd, toRemove, needsUpdate := h.taintsNeedUpdate(tt.nodeTaints, tt.expectedTaints)

			assert.Equal(t, tt.expectUpdate, needsUpdate, "needsUpdate mismatch")
			assert.Equal(t, tt.expectedToAddCount, len(toAdd), "toAdd count mismatch")
			assert.Equal(t, tt.expectedToRemoveCount, len(toRemove), "toRemove count mismatch")
		})
	}
}

func TestApplyTaintChanges(t *testing.T) {
	h := &handler{}

	tests := []struct {
		name           string
		currentTaints  []corev1.Taint
		toAdd          []corev1.Taint
		toRemove       []int
		expectedTaints []corev1.Taint
	}{
		{
			name:          "add taint to empty list",
			currentTaints: nil,
			toAdd: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			toRemove: nil,
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		{
			name: "remove taint",
			currentTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			toAdd:          nil,
			toRemove:       []int{0},
			expectedTaints: []corev1.Taint{},
		},
		{
			name: "preserve user taints while removing default",
			currentTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				{Key: "custom-taint", Effect: corev1.TaintEffectNoSchedule},
			},
			toAdd:    nil,
			toRemove: []int{0},
			expectedTaints: []corev1.Taint{
				{Key: "custom-taint", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		{
			name: "add and remove",
			currentTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/etcd", Effect: corev1.TaintEffectNoExecute},
			},
			toAdd: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			toRemove: []int{0},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.applyTaintChanges(tt.currentTaints, tt.toAdd, tt.toRemove)

			assert.Equal(t, len(tt.expectedTaints), len(result), "result length mismatch")

			for i, expected := range tt.expectedTaints {
				assert.Equalf(t, expected.Key, result[i].Key, "taint key mismatch at index %d", i)
				assert.Equalf(t, expected.Effect, result[i].Effect, "taint effect mismatch at index %d", i)
			}
		})
	}
}

func TestWorkerLabelLogic(t *testing.T) {
	tests := []struct {
		name               string
		machineHasWorker   bool
		nodeHasWorkerLabel bool   // whether the node has the worker label at all
		nodeWorkerValue    string // value of the label when present
		shouldAddLabel     bool
		shouldRemoveLabel  bool
		shouldNormalize    bool
	}{
		{
			name:               "add worker label",
			machineHasWorker:   true,
			nodeHasWorkerLabel: false,
			nodeWorkerValue:    "",
			shouldAddLabel:     true,
			shouldRemoveLabel:  false,
			shouldNormalize:    false,
		},
		{
			name:               "remove worker label",
			machineHasWorker:   false,
			nodeHasWorkerLabel: true,
			nodeWorkerValue:    "true",
			shouldAddLabel:     false,
			shouldRemoveLabel:  true,
			shouldNormalize:    false,
		},
		{
			name:               "no change - both have correct value",
			machineHasWorker:   true,
			nodeHasWorkerLabel: true,
			nodeWorkerValue:    "true",
			shouldAddLabel:     false,
			shouldRemoveLabel:  false,
			shouldNormalize:    false,
		},
		{
			name:               "no change - neither have",
			machineHasWorker:   false,
			nodeHasWorkerLabel: false,
			nodeWorkerValue:    "",
			shouldAddLabel:     false,
			shouldRemoveLabel:  false,
			shouldNormalize:    false,
		},
		{
			name:               "normalize empty value to true",
			machineHasWorker:   true,
			nodeHasWorkerLabel: true,
			nodeWorkerValue:    "", // label exists but empty
			shouldAddLabel:     false,
			shouldRemoveLabel:  false,
			shouldNormalize:    true,
		},
		{
			name:               "normalize wrong value to true",
			machineHasWorker:   true,
			nodeHasWorkerLabel: true,
			nodeWorkerValue:    "yes", // wrong value
			shouldAddLabel:     false,
			shouldRemoveLabel:  false,
			shouldNormalize:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from reconcileNodeMetadata
			node := &corev1.Node{}
			if node.Labels == nil {
				node.Labels = make(map[string]string)
			}

			if tt.nodeHasWorkerLabel {
				node.Labels[workerLabel] = tt.nodeWorkerValue
			}

			// Apply the actual controller logic
			workerLabelVal, hasWorkerLabel := node.Labels[workerLabel]
			needsUpdate := false

			if tt.machineHasWorker && (!hasWorkerLabel || workerLabelVal != "true") {
				// Add/normalize worker label
				node.Labels[workerLabel] = "true"
				needsUpdate = true
			} else if !tt.machineHasWorker && hasWorkerLabel {
				// Remove worker label
				delete(node.Labels, workerLabel)
				needsUpdate = true
			}

			if tt.shouldAddLabel || tt.shouldRemoveLabel || tt.shouldNormalize {
				assert.True(t, needsUpdate, "expected update but needsUpdate was false")
			} else {
				assert.False(t, needsUpdate, "expected no update but needsUpdate was true")
			}

			// Verify final state
			if tt.machineHasWorker {
				assert.Equal(t, "true", node.Labels[workerLabel], "worker label should be 'true'")
			} else {
				_, exists := node.Labels[workerLabel]
				assert.False(t, exists, "worker label should not be present")
			}
		})
	}
}

const (
	testMgmtClusterName = "c-m-abc123"
	testCAPIClusterName = "my-cluster"
	testCAPIClusterNS   = "fleet-default"
)

func TestCapiClusterRef(t *testing.T) {
	mgmtCluster := func(labels map[string]string) *apimgmtv3.Cluster {
		return &apimgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: testMgmtClusterName, Labels: labels},
		}
	}
	provClusters := func(clusters ...*provv1.Cluster) []*provv1.Cluster { return clusters }

	tests := []struct {
		name          string
		cluster       *apimgmtv3.Cluster
		indexResult   []*provv1.Cluster
		indexErr      error
		expected      *types.NamespacedName
		expectedError bool
	}{
		{
			name: "provisioning cluster backs the management cluster",
			// The CAPI cluster is named after the provisioning cluster, never after the
			// management cluster.
			cluster: mgmtCluster(nil),
			indexResult: provClusters(&provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: testCAPIClusterName, Namespace: testCAPIClusterNS},
			}),
			expected: &types.NamespacedName{Namespace: testCAPIClusterNS, Name: testCAPIClusterName},
		},
		{
			name: "turtles-imported CAPI cluster resolves through owner labels",
			cluster: mgmtCluster(map[string]string{
				capr.CAPIClusterOwnerLabel:   testCAPIClusterName,
				capr.CAPIClusterOwnerNSLabel: testCAPIClusterNS,
			}),
			expected: &types.NamespacedName{Namespace: testCAPIClusterNS, Name: testCAPIClusterName},
		},
		{
			name: "only one owner label is an error",
			cluster: mgmtCluster(map[string]string{
				capr.CAPIClusterOwnerLabel: testCAPIClusterName,
			}),
			expectedError: true,
		},
		{
			name:        "no provisioning cluster means no CAPI backing",
			cluster:     mgmtCluster(nil),
			indexResult: []*provv1.Cluster{},
			expected:    nil,
		},
		{
			name:    "multiple provisioning clusters is an error",
			cluster: mgmtCluster(nil),
			indexResult: provClusters(
				&provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: testCAPIClusterNS}},
				&provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: testCAPIClusterNS}},
			),
			expectedError: true,
		},
		{
			name:          "index error is propagated",
			cluster:       mgmtCluster(nil),
			indexErr:      fmt.Errorf("cache not synced"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			provClusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
			if tt.indexResult != nil || tt.indexErr != nil {
				provClusterCache.EXPECT().
					GetByIndex(provcluster.ByCluster, testMgmtClusterName).
					Return(tt.indexResult, tt.indexErr)
			}

			ref, err := capiClusterRef(tt.cluster, provClusterCache)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, ref)
		})
	}
}

func TestOnMachineChange(t *testing.T) {
	now := metav1.Now()
	machine := func(namespace, clusterName string, roles ...string) *capi.Machine {
		labels := map[string]string{capi.ClusterNameLabel: clusterName}
		for _, role := range roles {
			labels[role] = "true"
		}
		return &capi.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: "machine-1", Namespace: namespace, Labels: labels},
			Status: capi.MachineStatus{
				Conditions: []metav1.Condition{{
					Type:   capi.InfrastructureReadyCondition,
					Status: metav1.ConditionTrue,
				}},
				NodeRef: capi.MachineNodeReference{Name: "node-1"},
			},
		}
	}

	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: testCAPIClusterName, Namespace: testCAPIClusterNS},
		Spec: capi.ClusterSpec{
			ControlPlaneRef: capi.ContractVersionedObjectReference{
				APIGroup: capr.RKEAPIGroup,
				Kind:     rkeControlPlaneKind,
				Name:     testCAPIClusterName,
			},
		},
	}

	tests := []struct {
		name           string
		machine        *capi.Machine
		capiCluster    *capi.Cluster
		capiClusterErr error
		expectUpdate   bool
	}{
		{
			// Regression test: the machine is labelled with the CAPI cluster name, which is
			// not the management cluster name the handler used to compare against.
			name:         "machine of this cluster is reconciled",
			machine:      machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel),
			capiCluster:  capiCluster,
			expectUpdate: true,
		},
		{
			name:    "machine of another cluster is ignored",
			machine: machine(testCAPIClusterNS, "another-cluster", capr.ControlPlaneRoleLabel),
		},
		{
			name:    "machine in another namespace is ignored",
			machine: machine("fleet-other", testCAPIClusterName, capr.ControlPlaneRoleLabel),
		},
		{
			name:    "machine whose cluster is not RKE-backed is ignored",
			machine: machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel),
			capiCluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: testCAPIClusterName, Namespace: testCAPIClusterNS},
				Spec: capi.ClusterSpec{
					ControlPlaneRef: capi.ContractVersionedObjectReference{
						APIGroup: "controlplane.cluster.x-k8s.io",
						Kind:     "KubeadmControlPlane",
						Name:     testCAPIClusterName,
					},
				},
			},
		},
		{
			// The cluster being deleted should not requeue the machine forever.
			name:           "missing CAPI cluster is skipped",
			machine:        machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel),
			capiClusterErr: apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, testCAPIClusterName),
		},
		{
			name: "machine without a node reference is skipped",
			machine: func() *capi.Machine {
				m := machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel)
				m.Status.NodeRef = capi.MachineNodeReference{}
				return m
			}(),
			capiCluster: capiCluster,
		},
		{
			name: "machine whose infrastructure is not ready is skipped",
			machine: func() *capi.Machine {
				m := machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel)
				m.Status.Conditions = nil
				return m
			}(),
			capiCluster: capiCluster,
		},
		{
			name: "deleted machine is skipped",
			machine: func() *capi.Machine {
				m := machine(testCAPIClusterNS, testCAPIClusterName)
				m.DeletionTimestamp = &now
				return m
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
			rkeControlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
			nodeCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Node](ctrl)
			nodeClient := fake.NewMockNonNamespacedClientInterface[*corev1.Node, *corev1.NodeList](ctrl)

			// Any cache access for a machine that should have been filtered out is a failure:
			// gomock fails on calls without a matching expectation.
			if tt.capiCluster != nil || tt.capiClusterErr != nil {
				capiClusterCache.EXPECT().
					Get(testCAPIClusterNS, testCAPIClusterName).
					Return(tt.capiCluster, tt.capiClusterErr)
			}
			if tt.expectUpdate {
				nodeCache.EXPECT().Get("node-1").Return(&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				}, nil)
				rkeControlPlaneCache.EXPECT().Get(testCAPIClusterNS, testCAPIClusterName).Return(&rkev1.RKEControlPlane{
					Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.32.3+rke2r1"},
				}, nil)
				nodeClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(node *corev1.Node) (*corev1.Node, error) {
					assert.Equal(t, []corev1.Taint{capr.DefaultTaints[capr.DefaultTaintControlPlane]}, node.Spec.Taints)
					return node, nil
				})
			}

			h := &handler{
				capiCluster:          types.NamespacedName{Namespace: testCAPIClusterNS, Name: testCAPIClusterName},
				nodeClient:           nodeClient,
				nodeCache:            nodeCache,
				capiClusterCache:     capiClusterCache,
				rkeControlPlaneCache: rkeControlPlaneCache,
			}

			_, err := h.OnMachineChange("", tt.machine)
			assert.NoError(t, err)
		})
	}
}
