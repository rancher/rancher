package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var errRoleNotFound = apierrors.NewNotFound(schema.GroupResource{}, "")

func TestOnNamespaceChangeSkipsIrrelevantNamespaces(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ns *corev1.Namespace
	}{
		"nil namespace": {
			ns: nil,
		},
		"namespace being deleted": {
			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "testns3",
					DeletionTimestamp: &metav1.Time{},
				},
			},
		},
		"namespace terminating": {
			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "testns3"},
				Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			// no cache or client calls are expected at all
			h := &inheritedNamespacedRulesHandler{
				grCache:      wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				grbCache:     wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl),
				roles:        wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl),
				roleBindings: wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
				rbCache:      wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
				clusterName:  "c-m-test",
			}

			obj, err := h.onNamespaceChange("testns3", test.ns)
			assert.NoError(t, err)
			assert.Equal(t, test.ns, obj)
		})
	}
}

func TestOnNamespaceChangeNoMatchingGlobalRoles(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, "testns3").Return(nil, nil)

	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl),
		roles:        wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl),
		roleBindings: wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
		rbCache:      wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}

func TestOnNamespaceChangeGlobalRoleIndexError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, "testns3").Return(nil, fmt.Errorf("index error"))

	h := &inheritedNamespacedRulesHandler{
		grCache:     grCache,
		clusterName: "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.Error(t, err)
}

func TestOnNamespaceChangeCreatesRole(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	rules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get", "list"},
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

	wantRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/gr-owner": "nsgrole3",
			},
		},
		Rules: rules,
	}
	roles := wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	roles.EXPECT().Get("testns3", "nsgrole3-testns3", metav1.GetOptions{}).Return(nil, errRoleNotFound)
	roles.EXPECT().Create(wantRole).Return(wantRole, nil)

	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        roles,
		roleBindings: wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
		rbCache:      wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}

func TestOnNamespaceChangeUpdatesRoleWithDifferentRules(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	rules := []rbacv1.PolicyRule{{
		APIGroups: []string{"apps"},
		Resources: []string{"deployments"},
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

	existing := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/gr-owner": "nsgrole3",
			},
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get"},
		}},
	}
	wantRole := existing.DeepCopy()
	wantRole.Rules = rules

	roles := wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	roles.EXPECT().Get("testns3", "nsgrole3-testns3", metav1.GetOptions{}).Return(existing, nil)
	roles.EXPECT().Update(wantRole).Return(wantRole, nil)

	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        roles,
		roleBindings: wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
		rbCache:      wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}

func TestOnNamespaceChangeCreatesRoleBindingsForGRBs(t *testing.T) {
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
	grb := &v3.GlobalRoleBinding{
		ObjectMeta:     metav1.ObjectMeta{Name: "grb1"},
		GlobalRoleName: "nsgrole3",
		UserName:       "user-abc",
	}

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, "testns3").Return([]*v3.GlobalRole{gr}, nil)
	grbCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().GetByIndex(pkgrbac.GRBGlobalRoleIndex, "nsgrole3").Return([]*v3.GlobalRoleBinding{grb}, nil)

	roles := wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	roles.EXPECT().Get("testns3", "nsgrole3-testns3", metav1.GetOptions{}).Return(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/gr-owner": "nsgrole3",
			},
		},
		Rules: rules,
	}, nil)

	wantRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grb1-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/grb-owner": "grb1",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:     "User",
			Name:     "user-abc",
			APIGroup: rbacv1.GroupName,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "nsgrole3-testns3",
		},
	}
	rbCache := wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
	rbCache.EXPECT().Get("testns3", "grb1-testns3").Return(nil, errRoleNotFound)
	roleBindings := wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
	roleBindings.EXPECT().Create(wantRB).Return(wantRB, nil)

	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        roles,
		roleBindings: roleBindings,
		rbCache:      rbCache,
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}

func TestOnNamespaceChangeLeavesCorrectRoleBindingAlone(t *testing.T) {
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
	grb := &v3.GlobalRoleBinding{
		ObjectMeta:     metav1.ObjectMeta{Name: "grb1"},
		GlobalRoleName: "nsgrole3",
		UserName:       "user-abc",
	}

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, "testns3").Return([]*v3.GlobalRole{gr}, nil)
	grbCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().GetByIndex(pkgrbac.GRBGlobalRoleIndex, "nsgrole3").Return([]*v3.GlobalRoleBinding{grb}, nil)

	roles := wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	roles.EXPECT().Get("testns3", "nsgrole3-testns3", metav1.GetOptions{}).Return(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/gr-owner": "nsgrole3",
			},
		},
		Rules: rules,
	}, nil)

	existingRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grb1-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/grb-owner": "grb1",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:     "User",
			Name:     "user-abc",
			APIGroup: rbacv1.GroupName,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "nsgrole3-testns3",
		},
	}
	rbCache := wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
	rbCache.EXPECT().Get("testns3", "grb1-testns3").Return(existingRB, nil)

	// no Create or Delete expected on the RoleBinding client
	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        roles,
		roleBindings: wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
		rbCache:      rbCache,
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}

func TestOnNamespaceChangeRecreatesIncorrectRoleBinding(t *testing.T) {
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
	grb := &v3.GlobalRoleBinding{
		ObjectMeta:     metav1.ObjectMeta{Name: "grb1"},
		GlobalRoleName: "nsgrole3",
		UserName:       "user-abc",
	}

	grCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
	grCache.EXPECT().GetByIndex(pkgrbac.GRDownstreamNSIndex, "testns3").Return([]*v3.GlobalRole{gr}, nil)
	grbCache := wfakes.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().GetByIndex(pkgrbac.GRBGlobalRoleIndex, "nsgrole3").Return([]*v3.GlobalRoleBinding{grb}, nil)

	roles := wfakes.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	roles.EXPECT().Get("testns3", "nsgrole3-testns3", metav1.GetOptions{}).Return(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nsgrole3-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/gr-owner": "nsgrole3",
			},
		},
		Rules: rules,
	}, nil)

	// existing RoleBinding has the wrong subject; RoleRef is immutable so it must be recreated
	existingRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grb1-testns3",
			Namespace: "testns3",
			Labels: map[string]string{
				"authz.management.cattle.io/grb-owner": "grb1",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:     "User",
			Name:     "someone-else",
			APIGroup: rbacv1.GroupName,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "nsgrole3-testns3",
		},
	}
	wantRB := existingRB.DeepCopy()
	wantRB.Subjects = []rbacv1.Subject{{
		Kind:     "User",
		Name:     "user-abc",
		APIGroup: rbacv1.GroupName,
	}}

	rbCache := wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
	rbCache.EXPECT().Get("testns3", "grb1-testns3").Return(existingRB, nil)
	roleBindings := wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
	roleBindings.EXPECT().Delete("testns3", "grb1-testns3", gomock.Any()).Return(nil)
	roleBindings.EXPECT().Create(wantRB).Return(wantRB, nil)

	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        roles,
		roleBindings: roleBindings,
		rbCache:      rbCache,
		clusterName:  "c-m-test",
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testns3"}}
	_, err := h.onNamespaceChange("testns3", ns)
	assert.NoError(t, err)
}
