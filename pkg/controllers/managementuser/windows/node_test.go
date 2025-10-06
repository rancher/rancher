package windows

import (
	"fmt"
	"testing"

	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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
		"kubernetes.io/os": "windows",
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
	ctrl := gomock.NewController(t)
	n := &NodeTaintsController{
		nodeClient: newNodeClientMock(ctrl, cases),
	}
	for _, c := range cases {
		_, err := n.sync(c.node.Name, c.node)
		assert.Nilf(t, err, "failed to sync node, test case: %s", c.name)
		assert.Equal(t, c.shouldCall, c.called, "unexpected call update function, test case: %s", c.name)
	}
}

func newNodeClientMock(ctrl *gomock.Controller, cases []*testcase) corew.NodeClient {
	client := fake.NewMockNonNamespacedClientInterface[*v1.Node, *v1.NodeList](ctrl)
	client.EXPECT().Update(gomock.Any()).DoAndReturn(func(in1 *v1.Node) (*v1.Node, error) {
		for _, c := range cases {
			if c.node.Name == in1.Name {
				c.called = true
				return in1, nil
			}
		}
		return in1, fmt.Errorf("case not found, %+v", in1)
	}).AnyTimes()
	return client
}
