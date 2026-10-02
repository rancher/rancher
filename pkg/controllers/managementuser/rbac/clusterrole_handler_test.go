package rbac

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestClusterRoleHandlerSync covers the orphan ClusterRole sweep performed by crHandler.sync,
// including the nested-Rancher scenario where a ClusterRole was created by a different Rancher
// install (foreign ownership) and must not be deleted even if its RoleTemplate can't be resolved
// in this install's local RoleTemplate cache.
func TestClusterRoleHandlerSync(t *testing.T) {
	assert.NoError(t, settings.InstallUUID.Set("own-install-uuid"))

	tests := map[string]struct {
		obj            *rbacv1.ClusterRole
		roleTemplates  map[string]*v3.RoleTemplate
		expectDeletion bool
	}{
		"no owner annotation: left alone": {
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "not-owned"},
			},
			roleTemplates:  map[string]*v3.RoleTemplate{},
			expectDeletion: false,
		},
		"owning roleTemplate exists locally: left alone": {
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "rt-exists",
					Annotations: map[string]string{clusterRoleOwner: "rt-exists"},
				},
			},
			roleTemplates: map[string]*v3.RoleTemplate{
				"rt-exists": {ObjectMeta: metav1.ObjectMeta{Name: "rt-exists"}},
			},
			expectDeletion: false,
		},
		"legacy ClusterRole (no install-uuid annotation), roleTemplate missing: deleted": {
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "rt-missing-legacy",
					Annotations: map[string]string{clusterRoleOwner: "rt-missing-legacy"},
				},
			},
			roleTemplates:  map[string]*v3.RoleTemplate{},
			expectDeletion: true,
		},
		"own install-uuid, roleTemplate missing: deleted": {
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rt-missing-own",
					Annotations: map[string]string{
						clusterRoleOwner:            "rt-missing-own",
						clusterRoleOwnerInstallUUID: "own-install-uuid",
					},
				},
			},
			roleTemplates:  map[string]*v3.RoleTemplate{},
			expectDeletion: true,
		},
		"foreign install-uuid, roleTemplate missing: left alone": {
			obj: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rt-missing-foreign",
					Annotations: map[string]string{
						clusterRoleOwner:            "rt-missing-foreign",
						clusterRoleOwnerInstallUUID: "foreign-install-uuid",
					},
				},
			},
			roleTemplates:  map[string]*v3.RoleTemplate{},
			expectDeletion: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := newManager(withRoleTemplates(test.roleTemplates, nil, ctrl))

			crMock := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if test.expectDeletion {
				crMock.EXPECT().Delete(test.obj.Name, gomock.Any()).Return(nil)
			}
			m.clusterRoles = crMock

			h := newClusterRoleHandler(m)
			_, err := h.sync(test.obj.Name, test.obj)
			assert.NoError(t, err)
		})
	}
}
