package rbac

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	wname "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestOnNamespaceChangeRoleRefMatchesRoleForLongGlobalRoleName covers GlobalRole names long enough
// for SafeConcatName to truncate: the created RoleBinding must reference the Role by the exact name
// the Role is created with, or the binding grants nothing.
func TestOnNamespaceChangeRoleRefMatchesRoleForLongGlobalRoleName(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	grName := "my-custom-role-for-project-x-with-a-very-long-descriptive-name-here"
	nsName := "testns3"
	roleName := wname.SafeConcatName(grName, nsName)

	rules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get"},
	}}
	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{Name: grName},
		InheritedNamespacedRules: map[string][]rbacv1.PolicyRule{
			nsName: rules,
		},
	}
	grb := &v3.GlobalRoleBinding{
		ObjectMeta:     metav1.ObjectMeta{Name: "grb1"},
		GlobalRoleName: grName,
		UserName:       "user-abc",
	}

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, nsName).Return([]*v3.GlobalRole{gr}, nil)
	grbCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().GetByIndex(pkgrbac.GRBGlobalRoleIndex, grName).Return([]*v3.GlobalRoleBinding{grb}, nil)

	existingRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: nsName,
			Labels: map[string]string{
				pkgrbac.GrOwnerLabel: wname.SafeConcatName(grName),
			},
		},
		Rules: rules,
	}
	roleCache := wfakes.NewMockCacheInterface[*rbacv1.Role](ctrl)
	roleCache.EXPECT().Get(nsName, roleName).Return(existingRole, nil)

	rbName := wname.SafeConcatName(wname.SafeConcatName(grb.Name), nsName)
	rbCache := wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
	rbCache.EXPECT().Get(nsName, rbName).Return(nil, errRoleNotFound)

	var createdRB *rbacv1.RoleBinding
	roleBindings := wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
	roleBindings.EXPECT().Create(gomock.Any()).DoAndReturn(func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
		createdRB = rb
		return rb, nil
	})

	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl),
		roleCache:    roleCache,
		roleBindings: roleBindings,
		rbCache:      rbCache,
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
	_, err := h.onNamespaceChange(nsName, ns)
	assert.NoError(t, err)
	if assert.NotNil(t, createdRB) {
		assert.Equal(t, roleName, createdRB.RoleRef.Name)
	}
}

// TestOnNamespaceChangeSkipsAPIWhenCachedRoleCorrect covers the cache short-circuit: when the
// cached Role already matches the desired one, no API call is made for the Role at all.
func TestOnNamespaceChangeSkipsAPIWhenCachedRoleCorrect(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	rules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get"},
	}}
	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{Name: "nsgrole3"},
		InheritedNamespacedRules: map[string][]rbacv1.PolicyRule{
			"testns3": rules,
		},
	}

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, "testns3").Return([]*v3.GlobalRole{gr}, nil)
	grbCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().GetByIndex(pkgrbac.GRBGlobalRoleIndex, "nsgrole3").Return(nil, nil)

	roleCache := wfakes.NewMockCacheInterface[*rbacv1.Role](ctrl)
	roleCache.EXPECT().Get("testns3", "nsgrole3-testns3").Return(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				pkgrbac.GrOwnerLabel: "nsgrole3",
			},
		},
		Rules: rules,
	}, nil)

	// no expectations on the Role client: a matching cached Role must short-circuit all API calls
	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl),
		roleCache:    roleCache,
		roleBindings: wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
		rbCache:      wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}
