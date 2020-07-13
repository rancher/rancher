package nodepool

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rke/services"
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePrefix(t *testing.T) {

	tests := []struct {
		name          string
		fullPrefix    string
		wantPrefix    string
		wantMinLength int
		wantStart     int
	}{{
		name:          "prefix with an 2 digit integer",
		fullPrefix:    "my-worker25",
		wantPrefix:    "my-worker",
		wantMinLength: 2,
		wantStart:     25,
	}, {
		name:          "prefix with an 1 digit integer",
		fullPrefix:    "pool4",
		wantPrefix:    "pool",
		wantMinLength: 1,
		wantStart:     4,
	}, {
		name:          "default case",
		fullPrefix:    "genericNodepool",
		wantPrefix:    "genericNodepool",
		wantMinLength: 1,
		wantStart:     1,
	},
	}
	for _, tt := range tests {

		gotPrefix, gotMinLength, gotStart := parsePrefix(tt.fullPrefix)

		assert.Equal(t, tt.wantPrefix, gotPrefix)
		assert.Equal(t, tt.wantMinLength, gotMinLength)
		assert.Equal(t, tt.wantStart, gotStart)
	}
}

func Test_roleUpdate(t *testing.T) {
	var tests = []struct {
		name            string
		node            *v3.Node
		nodepool        *v3.NodePool
		want            bool
		nodeAfterUpdate *v3.Node
	}{
		{
			name: "all roles; nodepool & node",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeConfig: &rketypes.RKEConfigNode{
						Role: []string{services.ETCDRole, services.ControlRole, services.WorkerRole},
					},
				},
			},
			nodepool: &v3.NodePool{
				// per the types struct tags, these will always be defined and never be nil
				Spec: v3.NodePoolSpec{
					Etcd:         true,
					ControlPlane: true,
					Worker:       true,
				},
			},
			want:            false,
			nodeAfterUpdate: nil,
		},
		{
			name: "worker only",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeConfig: &rketypes.RKEConfigNode{
						Role: []string{services.WorkerRole},
					},
				},
			},
			nodepool: &v3.NodePool{
				Spec: v3.NodePoolSpec{
					Etcd:         false,
					ControlPlane: false,
					Worker:       true,
				},
			},
			want:            false,
			nodeAfterUpdate: nil,
		},
		{
			name: "worker => controlplane ",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeConfig: &rketypes.RKEConfigNode{
						Role: []string{services.WorkerRole},
					},
				},
			},
			nodepool: &v3.NodePool{
				Spec: v3.NodePoolSpec{
					Etcd:         true,
					ControlPlane: true,
					Worker:       true,
				},
			},
			want: true,
			nodeAfterUpdate: &v3.Node{
				Status: v3.NodeStatus{
					NodeConfig: &rketypes.RKEConfigNode{
						// order matters here
						Role: []string{services.ControlRole, services.ETCDRole, services.WorkerRole},
					},
				},
			},
		},
		{
			name: "controlplane => worker",
			node: &v3.Node{
				Status: v3.NodeStatus{
					NodeConfig: &rketypes.RKEConfigNode{
						Role: []string{services.WorkerRole, services.ETCDRole, services.ControlRole},
					},
				},
			},
			nodepool: &v3.NodePool{
				Spec: v3.NodePoolSpec{
					Etcd:         false,
					ControlPlane: false,
					Worker:       true,
				},
			},
			want: true,
			nodeAfterUpdate: &v3.Node{
				Status: v3.NodeStatus{
					NodeConfig: &rketypes.RKEConfigNode{
						// order matters here
						Role: []string{services.WorkerRole},
					},
				},
			},
		},
	}
	c := &Controller{}
	for _, tt := range tests {
		got := needRoleUpdate(tt.node, tt.nodepool)
		require.Equal(t, tt.want, got)
		if got {
			newNode, err := c.updateNodeRoles(tt.node, tt.nodepool, true)
			if err != nil {
				t.Errorf("error updating node role: %v", err)
			}
			assert.EqualValues(t, tt.nodeAfterUpdate, newNode)
		}
	}
}
