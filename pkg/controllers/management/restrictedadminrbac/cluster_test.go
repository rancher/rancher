// mocks created with the following commands
// mockgen --build_flags=--mod=mod -package restrictedadminrbac -destination ./mockIndexer_test.go k8s.io/client-go/tools/cache Indexer
package restrictedadminrbac

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var errExpected = errors.New("expected error")

func Test_rbaccontroller_clusterOwnerSync(t *testing.T) {
	localCluster := &v3.Cluster{}
	localCluster.Name = "local"
	downstream1 := &v3.Cluster{}
	downstream1.Name = "down1"
	downstream2 := &v3.Cluster{}
	downstream2.Name = "down2"

	baseGrb := &v3.GlobalRoleBinding{}
	baseGrb.Name = "baseGRB"
	baseGrb.GlobalRoleName = rbac.GlobalRestrictedAdmin
	baseGrb.APIVersion = "api/version"
	baseGrb.Kind = "CoolKind"
	baseGrb.UID = "123"
	baseGrb.UserName = "user32"
	baseGrb.GroupPrincipalName = "group29"

	groupCrtb := &v3.GlobalRoleBinding{}
	*groupCrtb = *baseGrb
	groupCrtb.UserName = ""

	tests := []struct {
		name    string
		setup   func(*mockController)
		grb     *v3.GlobalRoleBinding
		wantErr bool
	}{
		{
			name: "New Restricted Admin valid CRTB",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1}, nil)
				mc.mockCRTBCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.mockCRTBCtrl.EXPECT().Create(gomock.Any()).DoAndReturn(func(newCrtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					require.Equal(mc.t, newCrtb.ClusterName, downstream1.Name)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].APIVersion, baseGrb.APIVersion)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].Kind, baseGrb.Kind)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].Name, baseGrb.Name)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].UID, baseGrb.UID)
					require.Equal(mc.t, newCrtb.RoleTemplateName, "cluster-owner")
					require.Equal(mc.t, newCrtb.ClusterName, downstream1.Name)
					require.Equal(mc.t, newCrtb.Namespace, downstream1.Name)
					require.Equal(mc.t, newCrtb.Labels, map[string]string{sourceKey: grbHandlerName})
					require.Equal(mc.t, newCrtb.UserName, baseGrb.UserName)
					require.Equal(mc.t, newCrtb.UserPrincipalName, "")
					require.Equal(mc.t, newCrtb.GroupName, "")
					require.Equal(mc.t, newCrtb.GroupPrincipalName, "")
					return newCrtb, nil
				})
			},
		},
		{
			name: "skip local cluster",
			grb:  groupCrtb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1, localCluster}, nil)
				mc.mockCRTBCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.mockCRTBCtrl.EXPECT().Create(gomock.Any()).DoAndReturn(func(newCrtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					require.Equal(mc.t, newCrtb.ClusterName, downstream1.Name)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].APIVersion, baseGrb.APIVersion)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].Kind, baseGrb.Kind)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].Name, baseGrb.Name)
					require.Equal(mc.t, newCrtb.OwnerReferences[0].UID, baseGrb.UID)
					require.Equal(mc.t, newCrtb.RoleTemplateName, "cluster-owner")
					require.Equal(mc.t, newCrtb.ClusterName, downstream1.Name)
					require.Equal(mc.t, newCrtb.Namespace, downstream1.Name)
					require.Equal(mc.t, newCrtb.Labels, map[string]string{sourceKey: grbHandlerName})
					require.Equal(mc.t, newCrtb.UserName, "")
					require.Equal(mc.t, newCrtb.UserPrincipalName, "")
					require.Equal(mc.t, newCrtb.GroupName, "")
					require.Equal(mc.t, newCrtb.GroupPrincipalName, baseGrb.GroupPrincipalName)
					return newCrtb, nil
				})
			},
		},
		{
			name: "handle multiple cluster",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1, localCluster, downstream2}, nil)
				mc.mockCRTBCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "")).Times(2)
				mc.mockCRTBCtrl.EXPECT().Create(gomock.Any()).DoAndReturn(func(newCrtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					if newCrtb.ClusterName != downstream1.Name && newCrtb.ClusterName != downstream2.Name {
						require.FailNowf(mc.t, "Unknown cluster name for CRTB '%s'", newCrtb.ClusterName)
					}
					return newCrtb, nil
				}).Times(2)
			},
		},
		{
			name: "CRTB already created",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1}, nil)
				mc.mockCRTBCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			name: "One CRTB already created one not found",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1, downstream2}, nil)
				mc.mockCRTBCache.EXPECT().Get(downstream1.Name, gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.mockCRTBCache.EXPECT().Get(downstream2.Name, gomock.Any()).Return(nil, nil)
				mc.mockCRTBCtrl.EXPECT().Create(gomock.Any()).DoAndReturn(func(newCrtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					require.Equal(mc.t, newCrtb.ClusterName, downstream1.Name)
					return newCrtb, nil
				})
			},
		},
		{
			name:  "Nil Object",
			grb:   nil,
			setup: func(mc *mockController) {},
		},
		{
			name:  "Non restricted-admin GRB",
			grb:   &v3.GlobalRoleBinding{},
			setup: func(mc *mockController) {},
		},
		{
			name: "List Error",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return(nil, errExpected)
			},
			wantErr: true,
		},
		{
			name: "CRTB Cache Error",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1}, nil)
				mc.mockCRTBCache.EXPECT().Get(downstream1.Name, gomock.Any()).Return(nil, errExpected)
			},
			wantErr: true,
		},
		{
			name: "Create error",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream1}, nil)
				mc.mockCRTBCache.EXPECT().Get(downstream1.Name, gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.mockCRTBCtrl.EXPECT().Create(gomock.Any()).Return(nil, errExpected)
			},
			wantErr: true,
		},
		{
			name: "One Error One Create",
			grb:  baseGrb,
			setup: func(mc *mockController) {
				mc.mockClusterCache.EXPECT().List(gomock.Any()).Return([]*v3.Cluster{downstream2, downstream1}, nil)
				mc.mockCRTBCache.EXPECT().Get(downstream1.Name, gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.mockCRTBCache.EXPECT().Get(downstream2.Name, gomock.Any()).Return(nil, errExpected)
				mc.mockCRTBCtrl.EXPECT().Create(gomock.Any()).DoAndReturn(func(newCrtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					require.Equal(mc.t, newCrtb.ClusterName, downstream1.Name)
					return newCrtb, nil
				})
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := newMockController(t)
			tt.setup(mockCtrl)
			r := mockCtrl.rbacController()
			_, err := r.clusterOwnerSync("", tt.grb)

			if tt.wantErr {
				require.Error(t, err, "Expected an error from clusterOwnerSync")
				return
			}
			require.NoError(t, err, "Unexpected error from clusterOwnerSync")
		})
	}
}

func Test_enqueueGrbOnCRTB(t *testing.T) {
	testGrb := &v3.GlobalRoleBinding{}
	testGrb2 := &v3.GlobalRoleBinding{}
	testGrb.Name = "test-grb"
	testGrb2.Name = "test-grb2"

	baseCRTB := v3.ClusterRoleTemplateBinding{}
	baseCRTB.Name = "baseCrtb"

	tests := []struct {
		name     string
		setup    func(*mockController)
		crtb     func() runtime.Object
		wantErr  bool
		wantKeys []relatedresource.Key
	}{
		{
			name: "restricted-admin owner",
			setup: func(mc *mockController) {
				mc.mockGRBCache.EXPECT().Get(testGrb.Name).Return(testGrb, nil)
			},
			crtb: func() runtime.Object {
				ret := baseCRTB
				ret.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       testGrb.Name,
					},
				}
				return &ret
			},
			wantKeys: []relatedresource.Key{{Name: testGrb.Name}},
		},
		{
			name:  "wrong group version",
			setup: func(mc *mockController) {},
			crtb: func() runtime.Object {
				ret := baseCRTB
				ret.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: "v3",
						Kind:       "GlobalRoleBinding",
						Name:       testGrb.Name,
					},
				}
				return &ret
			},
		},
		{
			name:  "wrong kind",
			setup: func(mc *mockController) {},
			crtb: func() runtime.Object {
				ret := baseCRTB
				ret.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "RoleTemplate",
						Name:       testGrb.Name,
					},
				}
				return &ret
			},
		},
		{
			name: "multiple owners",
			setup: func(mc *mockController) {
				mc.mockGRBCache.EXPECT().Get(testGrb.Name).Return(testGrb, nil)
				mc.mockGRBCache.EXPECT().Get(testGrb2.Name).Return(testGrb2, nil)
			},
			crtb: func() runtime.Object {
				ret := baseCRTB
				ret.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       testGrb.Name,
					},
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       testGrb2.Name,
					},
				}
				return &ret
			},
			wantKeys: []relatedresource.Key{{Name: testGrb.Name}, {Name: testGrb2.Name}},
		},
		{
			name: "failed to get GRB",
			setup: func(mc *mockController) {
				mc.mockGRBCache.EXPECT().Get(testGrb.Name).Return(nil, errExpected)
			},
			crtb: func() runtime.Object {
				ret := baseCRTB
				ret.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       testGrb.Name,
					},
				}
				return &ret
			},
			wantErr: true,
		},
		{
			name: "GRB not found",
			setup: func(mc *mockController) {
				mc.mockGRBCache.EXPECT().Get(testGrb.Name).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
			},
			crtb: func() runtime.Object {
				ret := baseCRTB
				ret.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       testGrb.Name,
					},
				}
				return &ret
			},
		},
		{
			name:  "object not a CRTB",
			setup: func(mc *mockController) {},
			crtb: func() runtime.Object {
				return &v3.ProjectRoleTemplateBinding{}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := newMockController(t)
			tt.setup(mockCtrl)
			r := mockCtrl.rbacController()
			keys, err := r.enqueueGrbOnCRTB("", "", tt.crtb())
			if tt.wantErr {
				require.Error(t, err, "Expected an error from enqueueGrbOnCRTB")
				return
			}
			require.NoError(t, err, "Unexpected error from enqueueGrbOnCRTB")
			require.Equal(t, tt.wantKeys, keys, "incorrect related keys returned")
		})
	}
}

func Test_enqueueGrbOnCluster(t *testing.T) {
	testGrb := &v3.GlobalRoleBinding{}
	testGrb2 := &v3.GlobalRoleBinding{}
	testGrb.Name = "test-grb"
	testGrb2.Name = "test-grb2"

	localCluster := &v3.Cluster{}
	localCluster.Name = "local"
	downstream1 := &v3.Cluster{}
	downstream1.Name = "down1"
	deletedCluster := &v3.Cluster{}
	deletedCluster.Name = "deleted"
	deletedCluster.DeletionTimestamp = &v1.Time{}

	tests := []struct {
		name     string
		setup    func(*mockController)
		cluster  runtime.Object
		wantErr  bool
		wantKeys []relatedresource.Key
	}{
		{
			name: "downstream cluster changed",
			setup: func(mc *mockController) {
				mc.mockIndexer.EXPECT().ByIndex(grbByRoleIndex, rbac.GlobalRestrictedAdmin).Return([]any{testGrb, testGrb2}, nil)
			},
			cluster:  downstream1,
			wantKeys: []relatedresource.Key{{Name: testGrb.Name}, {Name: testGrb2.Name}},
		},
		{
			name: "failed to get grbs",
			setup: func(mc *mockController) {
				mc.mockIndexer.EXPECT().ByIndex(grbByRoleIndex, rbac.GlobalRestrictedAdmin).Return(nil, errExpected)
			},
			cluster: downstream1,
			wantErr: true,
		},
		{
			name: "local cluster changed",
			setup: func(mc *mockController) {
			},
			cluster: localCluster,
		},
		{
			name:    "non cluster object given",
			setup:   func(mc *mockController) {},
			cluster: testGrb,
		},
		{
			name:    "nil cluster given",
			setup:   func(mc *mockController) {},
			cluster: nil,
		},
		{
			name:    "cluster is being deleted",
			setup:   func(mc *mockController) {},
			cluster: deletedCluster,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := newMockController(t)
			tt.setup(mockCtrl)
			r := mockCtrl.rbacController()
			keys, err := r.enqueueGrbOnCluster("", "", tt.cluster)
			if tt.wantErr {
				require.Error(t, err, "Expected an error from enqueueGrbOnCluster")
				return
			}
			require.NoError(t, err, "Unexpected error from enqueueGrbOnCluster")
			require.Equal(t, tt.wantKeys, keys, "incorrect related keys returned")
		})
	}
}

func newMockController(t *testing.T) *mockController {
	t.Helper()
	ctrl := gomock.NewController(t)
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
	crtbCache := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
	crtbCtrl := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
	grbCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbIndexer := NewMockIndexer(ctrl)
	return &mockController{
		t:                t,
		mockIndexer:      grbIndexer,
		mockClusterCache: clusterCache,
		mockCRTBCache:    crtbCache,
		mockCRTBCtrl:     crtbCtrl,
		mockGRBCache:     grbCache,
	}
}

type mockController struct {
	t                *testing.T
	mockIndexer      *MockIndexer
	mockClusterCache *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
	mockCRTBCache    *fake.MockCacheInterface[*v3.ClusterRoleTemplateBinding]
	mockCRTBCtrl     *fake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]
	mockGRBCache     *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	mockRBInterface  *fakes.RoleBindingInterfaceMock
	mockRBLister     *fakes.RoleBindingListerMock
}

func (m *mockController) rbacController() *rbaccontroller {
	return &rbaccontroller{
		grbIndexer:   m.mockIndexer,
		clusterCache: m.mockClusterCache,
		crtbCache:    m.mockCRTBCache,
		crtbCtrl:     m.mockCRTBCtrl,
		grbCache:     m.mockGRBCache,
		roleBindings: m.mockRBInterface,
		rbLister:     m.mockRBLister,
	}
}
