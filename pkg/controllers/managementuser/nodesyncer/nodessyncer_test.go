package nodesyncer

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
