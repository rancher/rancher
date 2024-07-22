package authprovisioningv2

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rancher/kubernetes-provider-detector/providers"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
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
	err := errors.NewBadRequest("error")

	tests := map[string]struct {
		cluster       *v1.Cluster
		crtbCacheMock func(string) mgmtcontrollers.ClusterRoleTemplateBindingCache
		crtbMock      func() mgmtcontrollers.ClusterRoleTemplateBindingController
		expectedErr   error
	}{
		"no rke doesn't enqueue CRTBs": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.K3s,
					},
				},
			},
			crtbCacheMock: func(_ string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				return fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			},
			crtbMock: func() mgmtcontrollers.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			expectedErr: nil,
		},
		"rke enqueue CRTBs": {
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kubernetesprovider.ProviderKey: providers.RKE,
					},
				},
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
			crtbCacheMock: func(clusterName string) mgmtcontrollers.ClusterRoleTemplateBindingCache {
				mock := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
				mock.EXPECT().List(clusterName, labels.Everything()).Return(nil, err)
				return mock
			},
			crtbMock: func() mgmtcontrollers.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			expectedErr: err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			roleCacheMock := fake.NewMockCacheInterface[*rbacv1.Role](ctrl)
			roleCacheMock.EXPECT().Get(test.cluster.Namespace, clusterViewName(test.cluster)).Return(&rbacv1.Role{}, errors.NewNotFound(schema.GroupResource{}, ""))
			roleControllerMock := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
			roleControllerMock.EXPECT().Create(gomock.Any())
			h := handler{
				clusterRoleTemplateBindings:          test.crtbCacheMock(test.cluster.Name),
				clusterRoleTemplateBindingController: test.crtbMock(),
				roleCache:                            roleCacheMock,
				roleController:                       roleControllerMock,
			}

			_, err := h.OnCluster("", test.cluster)

			assert.Equal(t, err, test.expectedErr)
		})
	}
}
