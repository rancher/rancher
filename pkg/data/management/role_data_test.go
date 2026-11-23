package management

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/features"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddRolesManageOIDCClients(t *testing.T) {
	const roleName = "manage-oidc-clients"
	clientRule := rbacv1.PolicyRule{
		APIGroups: []string{"management.cattle.io"},
		Resources: []string{"oidcclients"},
		Verbs:     []string{"get", "list", "patch", "create", "update", "watch", "delete", "deletecollection"},
	}
	secretRules := map[string][]rbacv1.PolicyRule{
		"cattle-oidc-client-secrets": {{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
	for _, test := range []struct {
		name    string
		enabled bool
		upgrade bool
	}{
		{name: "fresh_install", enabled: true},
		{name: "upgrade", enabled: true, upgrade: true},
		{name: "feature_disabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			features.OIDCProvider.Set(test.enabled)
			t.Cleanup(features.OIDCProvider.Unset)

			ctrl := gomock.NewController(t)
			gr := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRole, *v3.GlobalRoleList](ctrl)
			rt := fake.NewMockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl)
			existing := &v3.GlobalRoleList{}
			var persisted *v3.GlobalRole
			if test.upgrade {
				existing.Items = []v3.GlobalRole{{
					ObjectMeta:  metav1.ObjectMeta{Name: roleName, Labels: defaultGRLabel},
					DisplayName: "Manage OIDC Clients",
					Builtin:     true,
					Rules:       []rbacv1.PolicyRule{clientRule},
				}}
				persisted = &existing.Items[0]
				gr.EXPECT().Update(ObjectMatcher(persisted)).DoAndReturn(func(role *v3.GlobalRole) (*v3.GlobalRole, error) {
					persisted = role
					return role, nil
				}).MaxTimes(1)
			}
			gr.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(gr, nil)
			gr.EXPECT().List(gomock.Any()).Return(existing, nil)
			gr.EXPECT().Create(gomock.Any()).DoAndReturn(func(role *v3.GlobalRole) (*v3.GlobalRole, error) {
				if role.Name == roleName {
					persisted = role
				}
				return role, nil
			}).AnyTimes()

			// Stop after GlobalRole reconciliation, before bootstrapping unrelated resources.
			stop := errors.New("stop before role templates")
			rt.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(nil, stop)
			_, err := addRoles(&wrangler.Context{Mgmt: &roleDataManagement{globalRole: gr, roleTemplate: rt}}, nil)
			require.ErrorIs(t, err, stop)
			if !test.enabled {
				assert.Nil(t, persisted)
				return
			}
			require.NotNil(t, persisted)
			assert.Equal(t, []rbacv1.PolicyRule{clientRule}, persisted.Rules)
			assert.Equal(t, secretRules, persisted.NamespacedRules, "OIDC client managers must be able to read generated client secrets in their dedicated namespace")
		})
	}
}

type roleDataManagement struct {
	managementv3.Interface
	globalRole   managementv3.GlobalRoleController
	roleTemplate managementv3.RoleTemplateController
}

func (m *roleDataManagement) GlobalRole() managementv3.GlobalRoleController {
	return m.globalRole
}

func (m *roleDataManagement) RoleTemplate() managementv3.RoleTemplateController {
	return m.roleTemplate
}
