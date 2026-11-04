package nodesyncer

import (
	"context"
	"errors"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type MockClusterLister struct {
	mock.Mock
}

func (m *MockClusterLister) Get(namespace, name string) (*v3.Cluster, error) {
	args := m.Called(namespace, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*v3.Cluster), args.Error(1)
}

func (m *MockClusterLister) List(namespace string, selector labels.Selector) (ret []*v3.Cluster, err error) {
	args := m.Called(namespace, selector)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*v3.Cluster), args.Error(1)
}

func TestReconcileAll_ClusterStatusEdgeCases(t *testing.T) {
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{Group: "cluster", Resource: "clusters"}, "c-testc")
	genericErr := errors.New("api connection timeout")
	testClusterNS := "ds-cluster"

	testCases := []struct {
		name             string
		clusterNamespace string
		setupMock        func(m *MockClusterLister)
		expectedErr      error
	}{
		{
			name:             "Cluster is not found, should return nil gracefully",
			clusterNamespace: testClusterNS,
			setupMock: func(m *MockClusterLister) {
				m.On("Get", "", testClusterNS).Return(nil, notFoundErr)
			},
			expectedErr: nil,
		},
		{
			name:             "Generic error occurs during restoration check, should bubble up error",
			clusterNamespace: testClusterNS,
			setupMock: func(m *MockClusterLister) {
				m.On("Get", "", testClusterNS).Return(nil, genericErr)
			},
			expectedErr: genericErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClusterLister := new(MockClusterLister)
			defer mockClusterLister.AssertExpectations(t)
			tc.setupMock(mockClusterLister)

			syncer := nodesSyncer{
				clusterNamespace: tc.clusterNamespace,
				clusterLister:    mockClusterLister,
			}

			err := syncer.reconcileAll()
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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
						"node-role.kubernetes.io/etcd":          "true",
						"node-role.kubernetes.io/controlplane":  "true",
						"node-role.kubernetes.io/control-plane": "true",
						"node-role.kubernetes.io/worker":        "true"},
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
						"node-role.kubernetes.io/etcd":          "true",
						"node-role.kubernetes.io/controlplane":  "true",
						"node-role.kubernetes.io/control-plane": "true",
						"node-role.kubernetes.io/worker":        "true"},
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
					NodeLabels: map[string]string{"node-role.kubernetes.io/control-plane": "true"},
				},
			},
			expectedNode: &v3.Node{
				Spec: v3.NodeSpec{
					Etcd:         false,
					ControlPlane: true,
					Worker:       false,
				},
				Status: v3.NodeStatus{
					NodeLabels: map[string]string{"node-role.kubernetes.io/control-plane": "true"},
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

func TestReconcileAllSkipsAClusterCreatedAgainUnderTheSameName(t *testing.T) {
	mockClusterLister := new(MockClusterLister)
	defer mockClusterLister.AssertExpectations(t)
	mockClusterLister.On("Get", "", "c-m-test").Return(&v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-new"}}, nil)

	// No node or machine listers: the machines of the new cluster must not be touched.
	syncer := nodesSyncer{
		clusterNamespace: "c-m-test",
		clusterUID:       "uid-old",
		clusterLister:    mockClusterLister,
	}

	assert.NoError(t, syncer.reconcileAll())
}

func TestNodeSyncerLeavesTheMachinesOfAClusterCreatedAgainUnderTheSameNameAlone(t *testing.T) {
	mockClusterLister := new(MockClusterLister)
	defer mockClusterLister.AssertExpectations(t)
	mockClusterLister.On("Get", "", "c-m-test").Return(&v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-new"}}, nil)

	// No machine client or cache: the new cluster's machines must not be looked up or updated.
	syncer := &nodeSyncer{
		clusterNamespace: "c-m-test",
		nodesSyncer:      &nodesSyncer{clusterNamespace: "c-m-test", clusterUID: "uid-old", clusterLister: mockClusterLister},
	}

	_, err := syncer.sync("node-1", &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}})

	assert.NoError(t, err)
}

func TestDrainLeavesTheMachineOfAClusterCreatedAgainUnderTheSameNameAlone(t *testing.T) {
	mockClusterLister := new(MockClusterLister)
	defer mockClusterLister.AssertExpectations(t)
	mockClusterLister.On("Get", "", "c-m-test").Return(&v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-new"}}, nil)

	// No machine client: the new cluster's machine must not be updated or requeued.
	d := &nodeDrain{clusterName: "c-m-test", clusterUID: "uid-old", clusterLister: mockClusterLister, nodesToContext: map[string]context.CancelFunc{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.drain(ctx, &v3.Node{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "m-1"}}, "node-1", cancel)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the drain should stop")
	}
}
