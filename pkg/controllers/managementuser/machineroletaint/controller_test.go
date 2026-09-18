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
		configured            map[string]bool
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
		{
			// The user explicitly configured node-role.kubernetes.io/control-plane=dedicated on
			// the pool. It is theirs, not an implicit default, so it must survive on a worker.
			name: "configured taint with a default key is kept on a worker",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Value: "dedicated", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints:        nil,
			configured:            map[string]bool{"node-role.kubernetes.io/control-plane:NoSchedule": true},
			expectUpdate:          false,
			expectedToAddCount:    0,
			expectedToRemoveCount: 0,
		},
		{
			// Same taint on a control-plane node: the default has the same key/effect but an
			// empty value, and must not overwrite the configured value.
			name: "configured taint with a default key is not normalized on a control plane node",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Value: "dedicated", Effect: corev1.TaintEffectNoSchedule},
			},
			expectedTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
			configured:            map[string]bool{"node-role.kubernetes.io/control-plane:NoSchedule": true},
			expectUpdate:          false,
			expectedToAddCount:    0,
			expectedToRemoveCount: 0,
		},
		{
			// A configured taint that differs in effect does not shield the default one.
			name: "configured taint with another effect does not shield the default",
			nodeTaints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				{Key: "node-role.kubernetes.io/control-plane", Value: "dedicated", Effect: corev1.TaintEffectNoExecute},
			},
			expectedTaints:        nil,
			configured:            map[string]bool{"node-role.kubernetes.io/control-plane:NoExecute": true},
			expectUpdate:          true,
			expectedToAddCount:    0,
			expectedToRemoveCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toAdd, toRemove, needsUpdate := h.taintsNeedUpdate(tt.nodeTaints, tt.expectedTaints, tt.configured)

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

func TestReconcileNodeMetadata(t *testing.T) {
	controlPlaneTaint := capr.DefaultTaints[capr.DefaultTaintControlPlane]
	etcdTaint := capr.DefaultTaints[capr.DefaultTaintEtcd]

	tests := []struct {
		name           string
		machineLabels  []string
		taintsAnn      string
		runtime        string
		nodeLabels     map[string]string
		nodeTaints     []corev1.Taint
		expectUpdate   bool
		expectedLabels map[string]string
		expectedTaints []corev1.Taint
	}{
		{
			name:           "worker label is added",
			machineLabels:  []string{capr.WorkerRoleLabel},
			expectUpdate:   true,
			expectedLabels: map[string]string{workerLabel: "true"},
			expectedTaints: nil,
		},
		{
			name:           "worker label is removed when the role is gone",
			machineLabels:  []string{capr.ControlPlaneRoleLabel},
			nodeLabels:     map[string]string{workerLabel: "true"},
			expectUpdate:   true,
			expectedLabels: map[string]string{},
			expectedTaints: []corev1.Taint{controlPlaneTaint},
		},
		{
			name:           "worker label value is normalized",
			machineLabels:  []string{capr.WorkerRoleLabel},
			nodeLabels:     map[string]string{workerLabel: "yes"},
			expectUpdate:   true,
			expectedLabels: map[string]string{workerLabel: "true"},
		},
		{
			name:           "worker node already labelled is left alone",
			machineLabels:  []string{capr.WorkerRoleLabel},
			nodeLabels:     map[string]string{workerLabel: "true"},
			expectUpdate:   false,
			expectedLabels: map[string]string{workerLabel: "true"},
		},
		{
			name:           "other labels are preserved",
			machineLabels:  []string{capr.WorkerRoleLabel},
			nodeLabels:     map[string]string{"custom": "value"},
			expectUpdate:   true,
			expectedLabels: map[string]string{"custom": "value", workerLabel: "true"},
		},
		{
			// Promoting a worker to control-plane+etcd: the worker label goes away and both
			// default taints appear.
			name:           "role change adds default taints and drops the worker label",
			machineLabels:  []string{capr.ControlPlaneRoleLabel, capr.EtcdRoleLabel},
			nodeLabels:     map[string]string{workerLabel: "true"},
			expectUpdate:   true,
			expectedLabels: map[string]string{},
			expectedTaints: []corev1.Taint{etcdTaint, controlPlaneTaint},
		},
		{
			// K3s does not get the etcd taint when the node is also control-plane.
			name:           "k3s combined control plane and etcd only gets the control plane taint",
			machineLabels:  []string{capr.ControlPlaneRoleLabel, capr.EtcdRoleLabel},
			runtime:        capr.RuntimeK3S,
			expectUpdate:   true,
			expectedLabels: map[string]string{},
			expectedTaints: []corev1.Taint{controlPlaneTaint},
		},
		{
			name:           "gaining the worker role removes the default taints",
			machineLabels:  []string{capr.ControlPlaneRoleLabel, capr.WorkerRoleLabel},
			nodeTaints:     []corev1.Taint{controlPlaneTaint, {Key: "custom", Effect: corev1.TaintEffectNoSchedule}},
			expectUpdate:   true,
			expectedLabels: map[string]string{workerLabel: "true"},
			expectedTaints: []corev1.Taint{{Key: "custom", Effect: corev1.TaintEffectNoSchedule}},
		},
		{
			// The user configured the control-plane key explicitly on the pool, so the taint is
			// theirs: it is neither removed on a worker nor rewritten with the default value.
			name:           "explicitly configured taint is preserved on a worker",
			machineLabels:  []string{capr.WorkerRoleLabel},
			taintsAnn:      `[{"key":"node-role.kubernetes.io/control-plane","value":"dedicated","effect":"NoSchedule"}]`,
			nodeTaints:     []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Value: "dedicated", Effect: corev1.TaintEffectNoSchedule}},
			expectUpdate:   true, // only for the worker label
			expectedLabels: map[string]string{workerLabel: "true"},
			expectedTaints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Value: "dedicated", Effect: corev1.TaintEffectNoSchedule}},
		},
		{
			name:           "explicitly configured taint is not normalized on a control plane node",
			machineLabels:  []string{capr.ControlPlaneRoleLabel},
			taintsAnn:      `[{"key":"node-role.kubernetes.io/control-plane","value":"dedicated","effect":"NoSchedule"}]`,
			nodeTaints:     []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Value: "dedicated", Effect: corev1.TaintEffectNoSchedule}},
			expectUpdate:   false,
			expectedLabels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			nodeClient := fake.NewMockNonNamespacedClientInterface[*corev1.Node, *corev1.NodeList](ctrl)

			machine := &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-1",
					Namespace: testCAPIClusterNS,
					Labels:    map[string]string{},
				},
			}
			for _, label := range tt.machineLabels {
				machine.Labels[label] = "true"
			}
			if tt.taintsAnn != "" {
				machine.Annotations = map[string]string{capr.TaintsAnnotation: tt.taintsAnn}
			}

			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: tt.nodeLabels},
				Spec:       corev1.NodeSpec{Taints: tt.nodeTaints},
			}

			var updated *corev1.Node
			if tt.expectUpdate {
				nodeClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(n *corev1.Node) (*corev1.Node, error) {
					updated = n
					return n, nil
				})
			}

			runtime := tt.runtime
			if runtime == "" {
				runtime = capr.RuntimeRKE2
			}

			h := &handler{nodeClient: nodeClient}
			assert.NoError(t, h.reconcileNodeMetadata(machine, node, runtime))

			if !tt.expectUpdate {
				return
			}
			assert.Equal(t, tt.expectedLabels, updated.Labels, "labels mismatch")
			assert.Equal(t, tt.expectedTaints, updated.Spec.Taints, "taints mismatch")
		})
	}
}

func TestConfiguredTaintKeys(t *testing.T) {
	machine := func(annotation string) *capi.Machine {
		m := &capi.Machine{ObjectMeta: metav1.ObjectMeta{Name: "machine-1", Namespace: testCAPIClusterNS}}
		if annotation != "" {
			m.Annotations = map[string]string{capr.TaintsAnnotation: annotation}
		}
		return m
	}

	keys, err := configuredTaintKeys(machine(""))
	assert.NoError(t, err)
	assert.Nil(t, keys)

	keys, err = configuredTaintKeys(machine(`[{"key":"a","effect":"NoSchedule"},{"key":"b","value":"v","effect":"NoExecute"}]`))
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"a:NoSchedule": true, "b:NoExecute": true}, keys)

	_, err = configuredTaintKeys(machine("not json"))
	assert.Error(t, err)
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
		name            string
		machine         *capi.Machine
		capiCluster     *capi.Cluster
		capiClusterErr  error
		expectRKELookup bool
		nodeErr         error
		expectUpdate    bool
	}{
		{
			// Regression test: the machine is labelled with the CAPI cluster name, which is
			// not the management cluster name the handler used to compare against.
			name:            "machine of this cluster is reconciled",
			machine:         machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel),
			capiCluster:     capiCluster,
			expectRKELookup: true,
			expectUpdate:    true,
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
			// Skipped before any cluster lookup: OnNodeChange picks the machine back up once
			// the node exists downstream.
			name: "machine without a node reference is skipped",
			machine: func() *capi.Machine {
				m := machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel)
				m.Status.NodeRef = capi.MachineNodeReference{}
				return m
			}(),
		},
		{
			name: "machine whose infrastructure is not ready is skipped",
			machine: func() *capi.Machine {
				m := machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel)
				m.Status.Conditions = nil
				return m
			}(),
		},
		{
			// The node has not reached the downstream informer yet. The machine must not error
			// out; OnNodeChange reconciles it when the node shows up.
			name:            "machine whose node is not in the downstream cache yet is skipped",
			machine:         machine(testCAPIClusterNS, testCAPIClusterName, capr.ControlPlaneRoleLabel),
			capiCluster:     capiCluster,
			expectRKELookup: true,
			nodeErr:         apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, "node-1"),
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
			if tt.expectRKELookup {
				rkeControlPlaneCache.EXPECT().Get(testCAPIClusterNS, testCAPIClusterName).Return(&rkev1.RKEControlPlane{
					Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.32.3+rke2r1"},
				}, nil)
				var node *corev1.Node
				if tt.nodeErr == nil {
					node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
				}
				nodeCache.EXPECT().Get("node-1").Return(node, tt.nodeErr)
			}
			if tt.expectUpdate {
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

// TestOnNodeChange covers the case OnMachineChange cannot handle on its own: the machine already
// has a NodeRef, but the downstream node only shows up (or comes back) afterwards, with no further
// machine event to trigger a reconcile.
func TestOnNodeChange(t *testing.T) {
	now := metav1.Now()
	node := func(annotations map[string]string) *corev1.Node {
		return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Annotations: annotations}}
	}
	linkedNode := node(map[string]string{
		capi.MachineAnnotation:          "machine-1",
		capi.ClusterNamespaceAnnotation: testCAPIClusterNS,
	})
	machine := &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-1",
			Namespace: testCAPIClusterNS,
			Labels: map[string]string{
				capi.ClusterNameLabel:      testCAPIClusterName,
				capr.ControlPlaneRoleLabel: "true",
			},
		},
		Status: capi.MachineStatus{
			Conditions: []metav1.Condition{{
				Type:   capi.InfrastructureReadyCondition,
				Status: metav1.ConditionTrue,
			}},
			NodeRef: capi.MachineNodeReference{Name: "node-1"},
		},
	}

	tests := []struct {
		name              string
		node              *corev1.Node
		machine           *capi.Machine
		machineErr        error
		expectMachineLook bool
		expectUpdate      bool
	}{
		{
			name:              "node linked to a machine of this cluster is reconciled",
			node:              linkedNode,
			machine:           machine,
			expectMachineLook: true,
			expectUpdate:      true,
		},
		{
			name: "node without the machine annotation is ignored",
			node: node(nil),
		},
		{
			name: "node annotated for another cluster namespace is ignored",
			node: node(map[string]string{
				capi.MachineAnnotation:          "machine-1",
				capi.ClusterNamespaceAnnotation: "fleet-other",
			}),
		},
		{
			name:              "missing machine is skipped",
			node:              linkedNode,
			machineErr:        apierrors.NewNotFound(schema.GroupResource{Resource: "machines"}, "machine-1"),
			expectMachineLook: true,
		},
		{
			name: "machine of another cluster is ignored",
			node: linkedNode,
			machine: func() *capi.Machine {
				m := machine.DeepCopy()
				m.Labels[capi.ClusterNameLabel] = "another-cluster"
				return m
			}(),
			expectMachineLook: true,
		},
		{
			name: "machine whose infrastructure is not ready is skipped",
			node: linkedNode,
			machine: func() *capi.Machine {
				m := machine.DeepCopy()
				m.Status.Conditions = nil
				return m
			}(),
			expectMachineLook: true,
		},
		{
			name: "deleted node is skipped",
			node: func() *corev1.Node {
				n := linkedNode.DeepCopy()
				n.DeletionTimestamp = &now
				return n
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			machineCache := fake.NewMockCacheInterface[*capi.Machine](ctrl)
			capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
			rkeControlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
			nodeClient := fake.NewMockNonNamespacedClientInterface[*corev1.Node, *corev1.NodeList](ctrl)

			if tt.expectMachineLook {
				machineCache.EXPECT().Get(testCAPIClusterNS, "machine-1").Return(tt.machine, tt.machineErr)
			}
			if tt.expectUpdate {
				capiClusterCache.EXPECT().Get(testCAPIClusterNS, testCAPIClusterName).Return(&capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: testCAPIClusterName, Namespace: testCAPIClusterNS},
					Spec: capi.ClusterSpec{
						ControlPlaneRef: capi.ContractVersionedObjectReference{
							APIGroup: capr.RKEAPIGroup,
							Kind:     rkeControlPlaneKind,
							Name:     testCAPIClusterName,
						},
					},
				}, nil)
				rkeControlPlaneCache.EXPECT().Get(testCAPIClusterNS, testCAPIClusterName).Return(&rkev1.RKEControlPlane{
					Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.32.3+rke2r1"},
				}, nil)
				nodeClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(n *corev1.Node) (*corev1.Node, error) {
					assert.Equal(t, []corev1.Taint{capr.DefaultTaints[capr.DefaultTaintControlPlane]}, n.Spec.Taints)
					return n, nil
				})
			}

			h := &handler{
				capiCluster:          types.NamespacedName{Namespace: testCAPIClusterNS, Name: testCAPIClusterName},
				nodeClient:           nodeClient,
				machineCache:         machineCache,
				capiClusterCache:     capiClusterCache,
				rkeControlPlaneCache: rkeControlPlaneCache,
			}

			_, err := h.OnNodeChange("", tt.node)
			assert.NoError(t, err)
		})
	}
}
