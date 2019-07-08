package windows

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testcase struct {
	name       string
	node       *v3.Node
	shouldCall bool
}

var (
	windowsNodeLabel = map[string]string{
		"beta.kubernetes.io/os": "windows",
	}
	falseValue = false
)

func Test_nodeController(t *testing.T) {
	cases := []testcase{
		testcase{
			name: "test node with linux host labels",
			node: &v3.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "local",
				},
				Status: v3.NodeStatus{
					NodeLabels: HostOSLabels[0],
				},
			},
			shouldCall: true,
		},
		testcase{
			name: "test node with windows host labels",
			node: &v3.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "local",
				},
				Status: v3.NodeStatus{
					NodeLabels: windowsNodeLabel,
				},
			},
			shouldCall: false,
		},
		testcase{
			name: "test node with linux node labels and node taints",
			node: &v3.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "local",
				},
				Spec: v3.NodeSpec{
					InternalNodeSpec: v1.NodeSpec{
						Taints: []v1.Taint{
							nodeTaint,
						},
					},
				},
				Status: v3.NodeStatus{
					NodeLabels: HostOSLabels[0],
				},
			},
			shouldCall: false,
		},
		testcase{
			name: "test node with linux node labels and taints and the node is updated from the other controller, should not update the node yet.",
			node: &v3.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "local",
				},
				Spec: v3.NodeSpec{
					InternalNodeSpec:    v1.NodeSpec{},
					UpdateTaintsFromAPI: &falseValue,
				},
				Status: v3.NodeStatus{
					NodeLabels: HostOSLabels[0],
				},
			},
			shouldCall: false,
		},
	}
	n := &NodeTaintsController{
		nodeClient: NewNodeInterface(cases),
	}
	for _, c := range cases {
		_, err := n.sync(getNodeKey(c.node), c.node)
		assert.Nilf(t, err, "failed to sync node, test case: %s", c.name)
	}
}

func NewNodeInterface(cases []testcase) v3.NodeInterface {
	return &fakes.NodeInterfaceMock{
		UpdateFunc: func(in1 *v3.Node) (*v3.Node, error) {
			// Revert the update change to find the specific case. And find out it should be call or not.
			compareNode := in1.DeepCopy()
			compareNode.Spec.DesiredNodeTaints = nil
			compareNode.Spec.UpdateTaintsFromAPI = nil
			for _, c := range cases {
				if reflect.DeepEqual(c.node, compareNode) {
					if !c.shouldCall {
						return in1, errors.New("should never call update in this case")
					}
					return in1, nil
				}
			}
			return in1, fmt.Errorf("case not found, %+v", in1)
		},
	}
}

func getNodeKey(node *v3.Node) string {
	return fmt.Sprintf("%s/%s", node.Namespace, node.Name)
}
