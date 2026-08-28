package clusterdeploy

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// fakeNodeLister returns a fixed set of nodes, ignoring the selector.
type fakeNodeLister struct {
	nodes []*apimgmtv3.Node
}

func (f *fakeNodeLister) List(_ string, _ labels.Selector) ([]*apimgmtv3.Node, error) {
	return f.nodes, nil
}

func (f *fakeNodeLister) Get(_, _ string) (*apimgmtv3.Node, error) {
	return nil, nil
}

var _ v3.NodeLister = &fakeNodeLister{}

func controlPlaneNode(name string, taints ...corev1.Taint) *apimgmtv3.Node {
	return &apimgmtv3.Node{
		Name: name,
		Spec: apimgmtv3.NodeSpec{
			InternalNodeSpec: corev1.NodeSpec{Taints: taints},
		},
		Status: apimgmtv3.NodeStatus{
			NodeName:   name,
			NodeLabels: map[string]string{"node-role.kubernetes.io/control-plane": "true"},
		},
	}
}

func workerNode(name string, taints ...corev1.Taint) *apimgmtv3.Node {
	return &apimgmtv3.Node{
		Name: name,
		Spec: apimgmtv3.NodeSpec{
			InternalNodeSpec: corev1.NodeSpec{Taints: taints},
		},
		Status: apimgmtv3.NodeStatus{
			NodeName:   name,
			NodeLabels: map[string]string{"node-role.kubernetes.io/worker": "true"},
		},
	}
}

var (
	controlPlaneTaint  = corev1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}
	etcdTaint          = corev1.Taint{Key: "node-role.kubernetes.io/etcd", Effect: corev1.TaintEffectNoExecute}
	uninitializedTaint = corev1.Taint{Key: "node.cloudprovider.kubernetes.io/uninitialized", Value: "true", Effect: corev1.TaintEffectPreferNoSchedule}
	notReadyTaint      = corev1.Taint{Key: "node.kubernetes.io/not-ready", Effect: corev1.TaintEffectNoSchedule}
)

func TestGetControlPlaneTaints(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []*apimgmtv3.Node
		expected []corev1.Taint
	}{
		{
			name:     "no nodes",
			nodes:    nil,
			expected: nil,
		},
		{
			name: "only worker nodes are ignored",
			nodes: []*apimgmtv3.Node{
				workerNode("w1", corev1.Taint{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}),
			},
			expected: nil,
		},
		{
			name: "taints are deduplicated across control plane nodes",
			nodes: []*apimgmtv3.Node{
				controlPlaneNode("cp1", controlPlaneTaint, etcdTaint),
				controlPlaneNode("cp2", controlPlaneTaint, etcdTaint),
				controlPlaneNode("cp3", controlPlaneTaint, etcdTaint),
			},
			expected: []corev1.Taint{controlPlaneTaint, etcdTaint},
		},
		{
			name: "transient cloud provider taint is excluded",
			nodes: []*apimgmtv3.Node{
				controlPlaneNode("cp1", controlPlaneTaint, etcdTaint, uninitializedTaint),
				controlPlaneNode("cp2", controlPlaneTaint, etcdTaint),
				controlPlaneNode("cp3", controlPlaneTaint, etcdTaint),
			},
			expected: []corev1.Taint{controlPlaneTaint, etcdTaint},
		},
		{
			name: "transient kubernetes taint is excluded",
			nodes: []*apimgmtv3.Node{
				controlPlaneNode("cp1", controlPlaneTaint, notReadyTaint),
			},
			expected: []corev1.Taint{controlPlaneTaint},
		},
		{
			name: "result is sorted regardless of the order the taints are found in",
			nodes: []*apimgmtv3.Node{
				controlPlaneNode("cp1", etcdTaint),
				controlPlaneNode("cp2", controlPlaneTaint),
			},
			expected: []corev1.Taint{controlPlaneTaint, etcdTaint},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &clusterDeploy{nodeLister: &fakeNodeLister{nodes: tt.nodes}}
			got, err := cd.getControlPlaneTaints("c-m-test")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_clusterAgentCRTName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *apimgmtv3.Cluster
		expected string
	}{
		{
			// v2prov: the CAPR planner also renders this manifest and picks default-token,
			// so this controller has to agree or its apply rolls the agent.
			name: "v2prov cluster uses the default token",
			cluster: &apimgmtv3.Cluster{
				Name:        "c-m-abc12",
				Annotations: map[string]string{imported.AdministratedAnnotation: "true"}},
			expected: capr.DefaultClusterRegistrationTokenName,
		},
		{
			// The choice must not depend on Status.Driver, which flips to "imported" when
			// the cluster agent's tunnel is authorized.
			name: "v2prov cluster before the agent connects",
			cluster: &apimgmtv3.Cluster{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
				Status: apimgmtv3.ClusterStatus{Driver: ""},
			},
			expected: capr.DefaultClusterRegistrationTokenName,
		},
		{
			name: "imported cluster keeps the system token",
			cluster: &apimgmtv3.Cluster{
				Name: "c-abc12",
				Status:     apimgmtv3.ClusterStatus{Driver: "rke2"},
			},
			expected: systemCRTName,
		},
		{
			name: "CAPI/turtles cluster keeps the system token",
			cluster: &apimgmtv3.Cluster{
				Name:        "c-m-abc12",
				Annotations: map[string]string{imported.AdministratedAnnotation: "false"}},
			expected: systemCRTName,
		},
		{
			name:     "hosted cluster keeps the system token",
			cluster:  &apimgmtv3.Cluster{Name: "c-abc12"},
			expected: systemCRTName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, clusterAgentCRTName(tt.cluster))
		})
	}
}

// TestGetControlPlaneTaintsIsDeterministic guards against the derived toleration
// list being reordered between reconciles. The list is rendered verbatim into the
// cluster agent pod template, so a different order means a different
// pod-template-hash, a new ReplicaSet and an endless rollout loop.
func TestGetControlPlaneTaintsIsDeterministic(t *testing.T) {
	extraTaint := corev1.Taint{Key: "dedicated", Value: "infra", Effect: corev1.TaintEffectNoSchedule}
	nodes := []*apimgmtv3.Node{
		controlPlaneNode("cp1", controlPlaneTaint, etcdTaint, extraTaint),
		controlPlaneNode("cp2", controlPlaneTaint, etcdTaint, extraTaint),
		controlPlaneNode("cp3", controlPlaneTaint, etcdTaint, extraTaint),
	}
	cd := &clusterDeploy{nodeLister: &fakeNodeLister{nodes: nodes}}

	first, err := cd.getControlPlaneTaints("c-m-test")
	require.NoError(t, err)
	require.Len(t, first, 3)

	for i := 0; i < 1000; i++ {
		got, err := cd.getControlPlaneTaints("c-m-test")
		require.NoError(t, err)
		require.Equal(t, first, got, "control plane taints changed order on iteration %d", i)
	}
}
