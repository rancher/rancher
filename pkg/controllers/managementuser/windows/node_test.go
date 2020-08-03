package windows

import (
	"fmt"
	"testing"

	apicorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	fakes1 "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testcase struct {
	name       string
	node       *v1.Node
	called     bool
	shouldCall bool
}

var (
	windowsNodeLabel = map[string]string{
		"beta.kubernetes.io/os": "windows",
	}
)

func Test_nodeController(t *testing.T) {
	cases := []*testcase{
		&testcase{
			name: "test node with linux host labels",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: HostOSLabels[0],
				},
			},
			shouldCall: true,
		},
		&testcase{
			name: "test node with windows host labels",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: windowsNodeLabel,
				},
			},
			shouldCall: false,
		},
		&testcase{
			name: "test node with linux node labels and node taints",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: HostOSLabels[0],
				},
				Spec: v1.NodeSpec{
					Taints: []v1.Taint{
						nodeTaint,
					},
				},
			},
			shouldCall: false,
		},
	}
	n := &NodeTaintsController{
		nodeClient: NewNodeInterface(cases),
	}
	for _, c := range cases {
		_, err := n.sync(c.node.Name, c.node)
		assert.Nilf(t, err, "failed to sync node, test case: %s", c.name)
		assert.Equal(t, c.shouldCall, c.called, "unexpected call update function, test case: %s", c.name)
	}
}

func NewNodeInterface(cases []*testcase) apicorev1.NodeInterface {
	return &fakes1.NodeInterfaceMock{
		UpdateFunc: func(in1 *v1.Node) (*v1.Node, error) {
			for _, c := range cases {
				if c.node.Name == in1.Name {
					c.called = true
					return in1, nil
				}
			}
			return in1, fmt.Errorf("case not found, %+v", in1)
		},
	}
}
