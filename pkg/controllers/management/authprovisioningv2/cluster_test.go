package authprovisioningv2

import (
	"testing"
	"time"

	"github.com/rancher/kubernetes-provider-detector/providers"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	wranglerrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestOnCluster(t *testing.T) {
	crtbs := []*v3.ClusterRoleTemplateBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "crtb1",
				Namespace: "ns1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "crtb2",
				Namespace: "ns2",
			},
		},
	}
	prtbInCluster := &v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prtb1",
			Namespace: "ns1",
		},
		ProjectName: "cluster:project",
	}
	prtbs := []*v3.ProjectRoleTemplateBinding{
		prtbInCluster,
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prtb2",
				Namespace: "ns1",
			},
			ProjectName: "invalid",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prtb3",
				Namespace: "ns1",
			},
			ProjectName: "clusterB:project",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prtb4",
				Namespace: "ns1",
			},
			ProjectName: "",
		},
	}
	err := errors.NewBadRequest("error")

	tests := map[string]struct {
		cluster            *v1.Cluster
		roleCacheMock      func(*gomock.Controller, *v1.Cluster) wranglerrbacv1.RoleCache
		roleMock           func(*gomock.Controller) wranglerrbacv1.RoleController
		roleBindingMock    func(*gomock.Controller, *v1.Cluster) wranglerrbacv1.RoleBindingController
		clusterMock        func(*gomock.Controller, *v1.Cluster) provisioningcontrollers.ClusterController
		crtbCacheMock      func(*gomock.Controller, string) mgmtcontrollers.ClusterRoleTemplateBindingCache
		crtbMock           func(*gomock.Controller) mgmtcontrollers.ClusterRoleTemplateBindingController
		prtbCacheMock      func(*gomock.Controller, string) mgmtcontrollers.ProjectRoleTemplateBindingCache
		prtbMock           func(*gomock.Controller) mgmtcontrollers.ProjectRoleTemplateBindingController
		expectedErr        error
		expectedFinalizers []string
	}{
		"role exists, don't enqueue CRTBs nor PRTBs": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster",
					Namespace:  "fleet-default",
					Finalizers: []string{capiResourcesCleanupFinalizer},
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			roleCacheMock: func(ctrl *gomock.Controller, cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, nil)
				return mock
			},
			roleMock: func(ctrl *gomock.Controller) wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Update(gomock.Any())

				return mock
			},
			clusterMock: func(ctrl *gomock.Controller, _ *v1.Cluster) provisioningcontrollers.ClusterController {
				return fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl)
			},
			crtbCacheMock: func(ctrl *gomock.Controller, _ string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			},
			crtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			prtbCacheMock: func(ctrl *gomock.Controller, _ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
			},
			prtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ProjectRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			},
			expectedErr:        nil,
			expectedFinalizers: []string{capiResourcesCleanupFinalizer},
		},
		"no role, enqueue CRTBs and PRTBs": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster",
					Namespace:  "fleet-default",
					Finalizers: []string{capiResourcesCleanupFinalizer},
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			roleCacheMock: func(ctrl *gomock.Controller, cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			roleMock: func(ctrl *gomock.Controller) wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Create(gomock.Any())

				return mock
			},
			clusterMock: func(ctrl *gomock.Controller, _ *v1.Cluster) provisioningcontrollers.ClusterController {
				return fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl)
			},
			crtbCacheMock: func(ctrl *gomock.Controller, clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(crtbs, nil)
				return mock
			},
			crtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				for _, crtb := range crtbs {
					mock.EXPECT().Enqueue(crtb.Namespace, crtb.Name)
				}
				return mock
			},
			prtbCacheMock: func(ctrl *gomock.Controller, _ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				mock.EXPECT().List("", labels.Everything()).Return(prtbs, nil)
				return mock
			},
			prtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ProjectRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
				mock.EXPECT().Enqueue(prtbInCluster.Namespace, prtbInCluster.Name)
				return mock
			},
			expectedErr:        nil,
			expectedFinalizers: []string{capiResourcesCleanupFinalizer},
		},
		"enqueue CRTBs error": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster",
					Namespace:  "fleet-default",
					Finalizers: []string{capiResourcesCleanupFinalizer},
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			roleCacheMock: func(ctrl *gomock.Controller, cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			roleMock: func(ctrl *gomock.Controller) wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Create(gomock.Any())

				return mock
			},
			clusterMock: func(ctrl *gomock.Controller, _ *v1.Cluster) provisioningcontrollers.ClusterController {
				return fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl)
			},
			crtbCacheMock: func(ctrl *gomock.Controller, clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(nil, err)
				return mock
			},
			crtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			prtbCacheMock: func(ctrl *gomock.Controller, _ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
			},
			prtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ProjectRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			},
			expectedErr:        err,
			expectedFinalizers: []string{capiResourcesCleanupFinalizer},
		},
		"enqueue PRTBs error": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster",
					Namespace:  "fleet-default",
					Finalizers: []string{capiResourcesCleanupFinalizer},
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			roleCacheMock: func(ctrl *gomock.Controller, cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			roleMock: func(ctrl *gomock.Controller) wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Create(gomock.Any())

				return mock
			},
			clusterMock: func(ctrl *gomock.Controller, _ *v1.Cluster) provisioningcontrollers.ClusterController {
				return fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl)
			},
			crtbCacheMock: func(ctrl *gomock.Controller, clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(crtbs, nil)
				return mock
			},
			crtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				for _, crtb := range crtbs {
					mock.EXPECT().Enqueue(crtb.Namespace, crtb.Name)
				}
				return mock
			},
			prtbCacheMock: func(ctrl *gomock.Controller, _ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				mock.EXPECT().List("", labels.Everything()).Return(nil, err)
				return mock
			},
			prtbMock: func(ctrl *gomock.Controller) mgmtcontrollers.ProjectRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			},
			expectedErr:        err,
			expectedFinalizers: []string{capiResourcesCleanupFinalizer},
		},
		"deleting cluster cleans admin role bindings and removes finalizer": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster",
					Namespace:  "fleet-default",
					Finalizers: []string{capiResourcesCleanupFinalizer},
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
			},
			roleBindingMock: func(ctrl *gomock.Controller, cluster *v1.Cluster) wranglerrbacv1.RoleBindingController {
				mock := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
				mock.EXPECT().List(cluster.Namespace, metav1.ListOptions{}).Return(&rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: cluster.Namespace},
						RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: rbac.ProvisioningClusterAdminName(cluster)},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "keep", Namespace: cluster.Namespace},
						RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "other-role"},
					},
				}}, nil)
				mock.EXPECT().Delete(cluster.Namespace, "cleanup", &metav1.DeleteOptions{}).Return(nil)
				return mock
			},
			clusterMock: func(ctrl *gomock.Controller, _ *v1.Cluster) provisioningcontrollers.ClusterController {
				mock := fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl)
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(updated *v1.Cluster) (*v1.Cluster, error) {
					assert.Empty(t, updated.Finalizers)
					return updated, nil
				})
				return mock
			},
			expectedErr:        nil,
			expectedFinalizers: []string{},
		},
		"missing finalizer updates cluster before role handling": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "fleet-default",
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			clusterMock: func(ctrl *gomock.Controller, cluster *v1.Cluster) provisioningcontrollers.ClusterController {
				mock := fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl)
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(updated *v1.Cluster) (*v1.Cluster, error) {
					assert.Equal(t, []string{capiResourcesCleanupFinalizer}, updated.Finalizers)
					return updated, nil
				})
				return mock
			},
			expectedErr:        nil,
			expectedFinalizers: []string{capiResourcesCleanupFinalizer},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			clusterName := ""
			if test.cluster != nil {
				clusterName = test.cluster.Name
			}

			var roleCache wranglerrbacv1.RoleCache
			if test.roleCacheMock != nil {
				roleCache = test.roleCacheMock(ctrl, test.cluster)
			}

			var roleController wranglerrbacv1.RoleController
			if test.roleMock != nil {
				roleController = test.roleMock(ctrl)
			}

			var roleBindingController wranglerrbacv1.RoleBindingController
			if test.roleBindingMock != nil {
				roleBindingController = test.roleBindingMock(ctrl, test.cluster)
			}

			var clusterController provisioningcontrollers.ClusterController
			if test.clusterMock != nil {
				clusterController = test.clusterMock(ctrl, test.cluster)
			}

			var crtbCache mgmtcontrollers.ClusterRoleTemplateBindingCache
			if test.crtbCacheMock != nil {
				crtbCache = test.crtbCacheMock(ctrl, clusterName)
			}

			var crtbController mgmtcontrollers.ClusterRoleTemplateBindingController
			if test.crtbMock != nil {
				crtbController = test.crtbMock(ctrl)
			}

			var prtbCache mgmtcontrollers.ProjectRoleTemplateBindingCache
			if test.prtbCacheMock != nil {
				prtbCache = test.prtbCacheMock(ctrl, clusterName)
			}

			var prtbController mgmtcontrollers.ProjectRoleTemplateBindingController
			if test.prtbMock != nil {
				prtbController = test.prtbMock(ctrl)
			}

			h := handler{
				clusterRoleTemplateBindings:          crtbCache,
				clusterRoleTemplateBindingController: crtbController,
				projectRoleTemplateBindings:          prtbCache,
				projectRoleTemplateBindingController: prtbController,
				roleCache:                            roleCache,
				roleController:                       roleController,
				roleBindingController:                roleBindingController,
				clusterController:                    clusterController,
			}

			result, err := h.OnCluster("", test.cluster)

			assert.Equal(t, err, test.expectedErr)
			if test.cluster != nil {
				assert.Equal(t, test.expectedFinalizers, result.Finalizers)
			}
		})
	}
}
