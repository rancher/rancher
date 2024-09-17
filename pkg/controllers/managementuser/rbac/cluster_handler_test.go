// mocks created with the following commands
//
//	mockgen --build_flags=--mod=mod -package rbac -destination ./v3mgmntMocks_test.go github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3 ClusterInterface
//	mockgen --build_flags=--mod=mod -package rbac -destination ./v1rbacMocks_test.go github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1 ClusterRoleBindingInterface,ClusterRoleBindingLister
package rbac

import (
	"fmt"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

var (
	errNotFound     = apierrors.NewNotFound(schema.GroupResource{}, "")
	errAlreadyExist = apierrors.NewAlreadyExists(schema.GroupResource{}, "")
	errExpected     = fmt.Errorf("expected test error")
)

func Test_clusterHandler_sync(t *testing.T) {
	testGrbs := []*v32.GlobalRoleBinding{
		{ObjectMeta: metav1.ObjectMeta{Name: "GRB1"}, GlobalRoleName: rbac.GlobalAdmin},
		{ObjectMeta: metav1.ObjectMeta{Name: "GRB2"}, GlobalRoleName: rbac.GlobalAdmin},
		{ObjectMeta: metav1.ObjectMeta{Name: "GRB3"}, GlobalRoleName: rbac.GlobalAdmin},
	}
	testCluster := &v32.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "testCluster"}}
	falseCluster := &v32.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "falseCluster"}}
	falseCluster.Status.Conditions = []v32.ClusterCondition{
		{Type: v32.ClusterConditionType(v32.ClusterConditionGlobalAdminsSynced), Status: "False"},
	}
	trueCluster := &v32.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "preSet"}}
	trueCluster.Status.Conditions = []v32.ClusterCondition{
		{Type: v32.ClusterConditionType(v32.ClusterConditionGlobalAdminsSynced), Status: "True"},
	}
	tests := []struct {
		name        string
		setup       func(mocks *testMocks)
		clusterName string
		key         string
		cluster     *v32.Cluster
		wantErr     bool
	}{
		{
			name:        "single GRB condition not set",
			clusterName: testCluster.Name,
			key:         testCluster.Name,
			cluster:     testCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCache.Add(testGrbs[0])
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil).Times(1)
				m.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v32.Cluster) (*v32.Cluster, error) {
					require.Len(m.t, cluster.Status.Conditions, 1)
					require.Equal(m.t, string(v32.ClusterConditionGlobalAdminsSynced), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
					require.Equal(m.t, "True", string(cluster.Status.Conditions[0].Status), "expected true condition to be set")
					return cluster, nil
				})
			},
		},
		{
			name:        "single GRB condition false",
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCache.Add(testGrbs[0])
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil).Times(1)
				m.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v32.Cluster) (*v32.Cluster, error) {
					require.Len(m.t, cluster.Status.Conditions, 1)
					require.Equal(m.t, string(v32.ClusterConditionGlobalAdminsSynced), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
					require.Equal(m.t, "True", string(cluster.Status.Conditions[0].Status), "expected true condition to be set")
					return cluster, nil
				})
			},
		},
		{
			name:        "condition true",
			clusterName: trueCluster.Name,
			key:         trueCluster.Name,
			cluster:     trueCluster.DeepCopy(),
			setup:       func(m *testMocks) {},
		},
		{
			name:        "multiple GRBs",
			clusterName: testCluster.Name,
			key:         testCluster.Name,
			cluster:     testCluster.DeepCopy(),
			setup: func(m *testMocks) {
				for _, obj := range testGrbs {
					m.mockCache.Add(obj)
				}
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[1])).Return(nil, errNotFound)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[2])).Return(nil, errNotFound)

				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil).Times(3)

				m.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v32.Cluster) (*v32.Cluster, error) {
					require.Len(m.t, cluster.Status.Conditions, 1)
					require.Equal(m.t, string(v32.ClusterConditionGlobalAdminsSynced), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
					require.Equal(m.t, "True", string(cluster.Status.Conditions[0].Status), "expected true condition to be set")
					return cluster, nil
				})
			},
		},
		{
			name:        "multiple GRBs one CRB already exist in cache",
			clusterName: testCluster.Name,
			key:         testCluster.Name,
			cluster:     testCluster.DeepCopy(),
			setup: func(m *testMocks) {
				for _, obj := range testGrbs {
					m.mockCache.Add(obj)
				}
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[1])).Return(&rbacv1.ClusterRoleBinding{}, nil)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[2])).Return(nil, errNotFound)

				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil).Times(2)

				m.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v32.Cluster) (*v32.Cluster, error) {
					require.Len(m.t, cluster.Status.Conditions, 1)
					require.Equal(m.t, string(v32.ClusterConditionGlobalAdminsSynced), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
					require.Equal(m.t, "True", string(cluster.Status.Conditions[0].Status), "expected true condition to be set")
					return cluster, nil
				})
			},
		},
		{
			name:        "multiple GRBs one CRB already exist, but not in cache",
			clusterName: testCluster.Name,
			key:         testCluster.Name,
			cluster:     testCluster.DeepCopy(),
			setup: func(m *testMocks) {
				for _, obj := range testGrbs {
					m.mockCache.Add(obj)
				}
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[1])).Return(nil, errNotFound)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[2])).Return(nil, errNotFound)

				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil)
				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, errAlreadyExist)
				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil)

				m.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v32.Cluster) (*v32.Cluster, error) {
					require.Len(m.t, cluster.Status.Conditions, 1)
					require.Equal(m.t, string(v32.ClusterConditionGlobalAdminsSynced), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
					require.Equal(m.t, "True", string(cluster.Status.Conditions[0].Status), "expected true condition to be set")
					return cluster, nil
				})
			},
		},
		{
			name:        "no admin GRBs returned",
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v32.Cluster) (*v32.Cluster, error) {
					require.Len(m.t, cluster.Status.Conditions, 1)
					require.Equal(m.t, string(v32.ClusterConditionGlobalAdminsSynced), string(cluster.Status.Conditions[0].Type), "incorrect condition set")
					require.Equal(m.t, "True", string(cluster.Status.Conditions[0].Status), "expected true condition to be set")
					return cluster, nil
				})
			},
		},
		{
			name:        "failed indexer call",
			wantErr:     true,
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				// create empty indexer
				m.mockCache = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			},
		},
		{
			name:        "failed create call",
			wantErr:     true,
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCache.Add(testGrbs[0])
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, errExpected)
			},
		},
		{
			name:        "failed cache get",
			wantErr:     true,
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCache.Add(testGrbs[0])
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errExpected)
			},
		},
		{
			name:        "failed create multiple GRBs",
			wantErr:     true,
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCache.Add(testGrbs[0])
				m.mockCache.Add(testGrbs[1])
				m.mockCache.Add(testGrbs[2])

				// first GRB returns not found and is created
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[1])).Return(nil, errNotFound)

				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil)
				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, errExpected)
			},
		},
		{
			name:        "failed cache get multiple GRBs",
			wantErr:     true,
			clusterName: falseCluster.Name,
			key:         falseCluster.Name,
			cluster:     falseCluster.DeepCopy(),
			setup: func(m *testMocks) {
				m.mockCache.Add(testGrbs[0])
				m.mockCache.Add(testGrbs[1])
				m.mockCache.Add(testGrbs[2])

				// first GRB returns not found and is created
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[0])).Return(nil, errNotFound)

				// second GRB returns an error and the handler returns that error.
				m.mockLister.EXPECT().Get("", rbac.GrbCRBName(testGrbs[1])).Return(nil, errExpected)

				m.mockInt.EXPECT().Create(gomock.Any()).Return(nil, nil)
			},
		},
		{
			name:        "nil cluster",
			clusterName: testCluster.Name,
			key:         testCluster.Name,
			cluster:     nil,
			setup:       func(m *testMocks) {},
		},
		{
			name:        "cluster mismatch",
			clusterName: "unique",
			key:         testCluster.Name,
			cluster:     testCluster.DeepCopy(),
			setup:       func(m *testMocks) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newMocks(t)
			tt.setup(mocks)
			h := &clusterHandler{
				clusterName:   tt.clusterName,
				clusters:      mocks.mockCluster,
				userCRB:       mocks.mockInt,
				userCRBLister: mocks.mockLister,
				grbIndexer:    mocks.mockCache,
			}
			_, err := h.sync(tt.key, tt.cluster)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

type testMocks struct {
	t           *testing.T
	mockCluster *MockClusterInterface
	mockInt     *MockClusterRoleBindingInterface
	mockLister  *MockClusterRoleBindingLister
	mockCache   cache.Indexer
}

func newMocks(t *testing.T) *testMocks {
	t.Helper()
	ctrl := gomock.NewController(t)
	indexers := map[string]cache.IndexFunc{
		grbByRoleIndex: grbByRole,
	}
	mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	mockIndexer.AddIndexers(indexers)
	return &testMocks{
		t:           t,
		mockCluster: NewMockClusterInterface(ctrl),
		mockInt:     NewMockClusterRoleBindingInterface(ctrl),
		mockLister:  NewMockClusterRoleBindingLister(ctrl),
		mockCache:   mockIndexer,
	}
}
