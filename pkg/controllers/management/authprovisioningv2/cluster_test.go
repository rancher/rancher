package authprovisioningv2

import (
	"testing"

	"github.com/rancher/kubernetes-provider-detector/providers"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
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
	ctrl := gomock.NewController(t)
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
		cluster       *v1.Cluster
		roleCacheMock func(*v1.Cluster) wranglerrbacv1.RoleCache
		roleMock      func() wranglerrbacv1.RoleController
		crtbCacheMock func(string) mgmtcontrollers.ClusterRoleTemplateBindingCache
		crtbMock      func() mgmtcontrollers.ClusterRoleTemplateBindingController
		prtbCacheMock func(string) mgmtcontrollers.ProjectRoleTemplateBindingCache
		prtbMock      func() mgmtcontrollers.ProjectRoleTemplateBindingController
		expectedErr   error
	}{
		"role exists, don't enqueue CRTBs nor PRTBs": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			roleCacheMock: func(cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, nil)
				return mock
			},
			roleMock: func() wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Update(gomock.Any())

				return mock
			},
			crtbCacheMock: func(_ string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			},
			crtbMock: func() mgmtcontrollers.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			prtbCacheMock: func(_ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
			},
			prtbMock: func() mgmtcontrollers.ProjectRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			},
			expectedErr: nil,
		},
		"no role, enqueue CRTBs and PRTBs": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s, // distro is irrelevant here
					},
				},
			},
			roleCacheMock: func(cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			roleMock: func() wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Create(gomock.Any())

				return mock
			},
			crtbCacheMock: func(clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(crtbs, nil)
				return mock
			},
			crtbMock: func() mgmtcontrollers.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				for _, crtb := range crtbs {
					mock.EXPECT().Enqueue(crtb.Namespace, crtb.Name)
				}
				return mock
			},
			prtbCacheMock: func(_ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				mock.EXPECT().List("", labels.Everything()).Return(prtbs, nil)
				return mock
			},
			prtbMock: func() mgmtcontrollers.ProjectRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
				mock.EXPECT().Enqueue(prtbInCluster.Namespace, prtbInCluster.Name)
				return mock
			},
			expectedErr: nil,
		},
		"rke enqueue CRTBs error": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.RKE,
					},
				},
			},
			roleCacheMock: func(cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			roleMock: func() wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Create(gomock.Any())

				return mock
			},
			crtbCacheMock: func(clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(nil, err)
				return mock
			},
			crtbMock: func() mgmtcontrollers.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			prtbCacheMock: func(_ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
			},
			prtbMock: func() mgmtcontrollers.ProjectRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			},
			expectedErr: err,
		},
		"rke enqueue PRTBs error": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.RKE,
					},
				},
			},
			roleCacheMock: func(cluster *v1.Cluster) wranglerrbacv1.RoleCache {
				mock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
				mock.EXPECT().Get(cluster.Namespace, clusterViewName(cluster)).
					Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			roleMock: func() wranglerrbacv1.RoleController {
				mock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
				mock.EXPECT().Create(gomock.Any())

				return mock
			},
			crtbCacheMock: func(clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(crtbs, nil)
				return mock
			},
			crtbMock: func() mgmtcontrollers.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				for _, crtb := range crtbs {
					mock.EXPECT().Enqueue(crtb.Namespace, crtb.Name)
				}
				return mock
			},
			prtbCacheMock: func(_ string) mgmtcontrollers.ProjectRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
				mock.EXPECT().List("", labels.Everything()).Return(nil, err)
				return mock
			},
			prtbMock: func() mgmtcontrollers.ProjectRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			},
			expectedErr: err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			h := handler{
				clusterRoleTemplateBindings:          test.crtbCacheMock(test.cluster.Name),
				clusterRoleTemplateBindingController: test.crtbMock(),
				projectRoleTemplateBindings:          test.prtbCacheMock(test.cluster.Name),
				projectRoleTemplateBindingController: test.prtbMock(),
				roleCache:                            test.roleCacheMock(test.cluster),
				roleController:                       test.roleMock(),
			}

			_, err := h.OnCluster("", test.cluster)

			assert.Equal(t, err, test.expectedErr)
		})
	}
}
