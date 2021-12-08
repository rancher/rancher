package nodesyncer

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

func TestDetermineNodeRole(t *testing.T) {
	var tests = []struct {
		name         string
		node         *v3.Node
		expectedNode *v3.Node
	}{
		{
			name: "all node labels",
			node: &v3.Node{
				Spec: v3.NodeSpec{},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{
						"node-role.kubernetes.io/etcd":         "true",
						"node-role.kubernetes.io/controlplane": "true",
						"node-role.kubernetes.io/master":       "true",
						"node-role.kubernetes.io/worker":       "true"},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         true,
					ControlPlane: true,
					Worker:       true,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{
						"node-role.kubernetes.io/etcd":         "true",
						"node-role.kubernetes.io/controlplane": "true",
						"node-role.kubernetes.io/master":       "true",
						"node-role.kubernetes.io/worker":       "true"},
				},
			},
		},
		{
			name: "etcd node label",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/etcd": "true"},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         true,
					ControlPlane: false,
					Worker:       false,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/etcd": "true"},
				},
			},
		},
		{
			name: "controlplane node label",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/controlplane": "true"},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         false,
					ControlPlane: true,
					Worker:       false,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/controlplane": "true"},
				},
			},
		},
		{
			name: "master node label",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/master": "true"},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         false,
					ControlPlane: true,
					Worker:       false,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/master": "true"},
				},
			},
		},
		{
			name: "worker node label",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/worker": "true"},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         false,
					ControlPlane: false,
					Worker:       true,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/worker": "true"},
				},
			},
		},
		{
			name: "no node labels set",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         false,
					ControlPlane: false,
					Worker:       true,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{},
				},
			},
		},
	}
	for _, tt := range tests {
		determineNodeRoles(tt.node)
		assert.EqualValues(t, tt.expectedNode, tt.node)
	}
}
