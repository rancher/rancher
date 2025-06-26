package node

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsNodeForNode(t *testing.T) {

	tests := []struct {
		name    string
		node    *corev1.Node
		machine *v3.Node
		want    bool
	}{
		{
			name: "no node config",
			node: &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "not nil",
				},
				Spec: corev1.NodeSpec{},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "1.2.3.4.5",
						},
					},
				},
			},
			machine: &v3.Node{
				Status: v32.NodeStatus{
					NodeName: "",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {

		result := IsNodeForNode(tt.node, tt.machine)
		assert.Equal(t, tt.want, result)
	}
}
