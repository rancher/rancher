package nodesyncer

import (
	"fmt"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"

	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	fake3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/taints"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type syncTaintsTestCase struct {
	name                string
	machine             v3.Node
	node                v1.Node
	nodeShouldUpdate    bool
	nodeUpdated         bool
	machineShouldUpdate bool
	machineUpdated      bool
}

func TestSyncNodeTaints(t *testing.T) {
	falseValue := false
	testCases := []*syncTaintsTestCase{
		&syncTaintsTestCase{
			name:                "test taints equal",
			machineShouldUpdate: true,
			nodeShouldUpdate:    false,
			machine: v3.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test1", Labels: map[string]string{nodehelper.LabelNodeName: "test1"}},
				Status: v32.NodeStatus{
					Conditions: []v32.NodeCondition{
						v32.NodeCondition{
							Type:   v32.NodeConditionRegistered,
							Status: v1.ConditionTrue,
						},
					},
					NodeName: "test1",
				},
				Spec: v32.NodeSpec{
					DesiredNodeTaints: []v1.Taint{
						v1.Taint{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule},
					},
					UpdateTaintsFromAPI: &falseValue,
				},
			},
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec: v1.NodeSpec{
					Taints: []v1.Taint{
						v1.Taint{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule},
					},
				},
			},
		},
		&syncTaintsTestCase{
			name:                "test add taints",
			machineShouldUpdate: true,
			nodeShouldUpdate:    true,
			machine: v3.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test2", Labels: map[string]string{nodehelper.LabelNodeName: "test2"}},
				Status: v32.NodeStatus{
					Conditions: []v32.NodeCondition{
						v32.NodeCondition{
							Type:   v32.NodeConditionRegistered,
							Status: v1.ConditionTrue,
						},
					},
					NodeName: "test2",
				},
				Spec: v32.NodeSpec{
					DesiredNodeTaints: []v1.Taint{
						v1.Taint{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule},
					},
					UpdateTaintsFromAPI: &falseValue,
				},
			},
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test2"},
				Spec:       v1.NodeSpec{},
			},
		},
		&syncTaintsTestCase{
			name:                "test remove taints",
			machineShouldUpdate: true,
			nodeShouldUpdate:    true,
			machine: v3.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test3", Labels: map[string]string{nodehelper.LabelNodeName: "test3"}},
				Status: v32.NodeStatus{
					Conditions: []v32.NodeCondition{
						v32.NodeCondition{
							Type:   v32.NodeConditionRegistered,
							Status: v1.ConditionTrue,
						},
					},
					NodeName: "test3",
				},
				Spec: v32.NodeSpec{
					UpdateTaintsFromAPI: &falseValue,
				},
			},
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test3"},
				Spec: v1.NodeSpec{
					Taints: []v1.Taint{
						v1.Taint{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule},
					},
				},
			},
		},
	}
	ctrl := gomock.NewController(t)
	syncer := nodesSyncer{
		machines: &fake3.NodeInterfaceMock{
			UpdateFunc: getMachineUpdateFunc(t, testCases),
		},
		nodeLister: getNodeListerMock(t, ctrl, testCases),
		nodeClient: getNodeClientMock(t, ctrl, testCases),
	}
	for _, c := range testCases {
		if _, err := syncer.syncTaints(c.machine.Name, &c.machine); err != nil {
			t.Fatalf("test case %s failed, syncTaints return errors %s", c.name, err.Error())
		}
		if c.nodeShouldUpdate != c.nodeUpdated {
			t.Fatalf("test case %s failed, expect node update status is %v but got %v", c.name, c.nodeShouldUpdate, c.nodeUpdated)
		}
		if c.machineShouldUpdate != c.machineUpdated {
			t.Fatalf("test case %s failed, expect machine update status is %v but got %v", c.name, c.machineShouldUpdate, c.machineUpdated)
		}
	}
}

func getMachineUpdateFunc(t *testing.T, cases []*syncTaintsTestCase) func(*v3.Node) (*v3.Node, error) {
	machineSet := caseByMachine(t, cases)
	return func(in1 *v3.Node) (*v3.Node, error) {
		c := machineSet[in1.Name]
		c.machineUpdated = true
		c.machine = *in1
		if c.machine.Spec.DesiredNodeTaints != nil || c.machine.Spec.UpdateTaintsFromAPI != nil {
			t.Fatalf("test case %s failed, update machine in node taints syncer should set DesiredNodeTaints and UpdateTaintsFromAPI to nil", c.name)
		}
		return &c.machine, nil
	}
}
func getNodeListerMock(t *testing.T, ctrl *gomock.Controller, cases []*syncTaintsTestCase) corew.NodeCache {
	nodeSet := caseByNode(t, cases)
	lister := fake.NewMockNonNamespacedCacheInterface[*v1.Node](ctrl)
	lister.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v1.Node, error) {
		c, ok := nodeSet[name]
		if !ok {
			return nil, httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("node %s not found", name))
		}
		return &c.node, nil
	}).AnyTimes()
	return lister
}

func getNodeClientMock(t *testing.T, ctrl *gomock.Controller, cases []*syncTaintsTestCase) corew.NodeClient {
	nodeSet := caseByNode(t, cases)
	client := fake.NewMockNonNamespacedClientInterface[*v1.Node, *v1.NodeList](ctrl)
	client.EXPECT().Update(gomock.Any()).DoAndReturn(func(in1 *v1.Node) (*v1.Node, error) {
		c, ok := nodeSet[in1.Name]
		if !ok {
			return nil, httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("node %s not found", in1.Name))
		}
		toAdd, toDel := taints.GetToDiffTaints(in1.Spec.Taints, c.machine.Spec.DesiredNodeTaints)
		if len(toAdd) != 0 || len(toDel) != 0 {
			return nil, fmt.Errorf("test case %s failed, node taints are different from machine taints", c.name)
		}
		c.nodeUpdated = true
		c.node = *in1
		return in1, nil
	}).AnyTimes()
	return client
}

func caseByMachine(t *testing.T, cases []*syncTaintsTestCase) map[string]*syncTaintsTestCase {
	rtn := map[string]*syncTaintsTestCase{}
	for _, c := range cases {
		if _, ok := rtn[c.machine.Name]; ok {
			t.Fatalf("test case %s has duplicated machine name %s", c.name, c.machine.Name)
		}
		rtn[c.machine.Name] = c
	}
	return rtn
}

func caseByNode(t *testing.T, cases []*syncTaintsTestCase) map[string]*syncTaintsTestCase {
	rtn := map[string]*syncTaintsTestCase{}
	for _, c := range cases {
		if _, ok := rtn[c.node.Name]; ok {
			t.Fatalf("test case %s has duplicated node name %s", c.name, c.node.Name)
		}
		rtn[c.node.Name] = c
	}
	return rtn
}
