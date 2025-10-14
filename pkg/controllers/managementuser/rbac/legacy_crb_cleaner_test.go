package rbac

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCRBSyncChecksGlobalAdminRole(t *testing.T) {
	ctrl := gomock.NewController(t)

	grCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.GlobalRole](ctrl)
	grCache.EXPECT().List(gomock.Any()).Return([]*apiv3.GlobalRole{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "custom-admin",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
				{
					NonResourceURLs: []string{"*"},
					Verbs:           []string{"*"},
				},
			},
		},
	}, nil).AnyTimes()

	grbCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().List(gomock.Any()).Return([]*apiv3.GlobalRoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "grb-8mlfc",
			},
			GlobalRoleName: "custom-admin",
			UserName:       "u-hgk79",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "grb-67psm",
			},
			GlobalRoleName:     "custom-admin",
			GroupPrincipalName: "okta_group://admins",
		},
	}, nil).AnyTimes()

	crbs := fake.NewMockNonNamespacedClientInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
	crbs.EXPECT().Update(gomock.Any()).DoAndReturn(func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
		return crb.DeepCopy(), nil
	}).AnyTimes()

	crbCleaner := &crbCleaner{
		noRemainingOwnerLabels: func(*rbacv1.ClusterRoleBinding) (bool, error) { return false, nil },
		crbs:                   crbs,
		grCache:                grCache,
		grbCache:               grbCache,
	}

	t.Run("user admin", func(t *testing.T) {
		crb, err := crbCleaner.sync("globaladmin-u-hgk79", &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "globaladmin-u-hgk79",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     rbac.ClusterAdminRoleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "u-hgk79",
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, crb)
		assert.Contains(t, crb.Annotations, rbac.CrbAdminGlobalRoleCheckedAnnotation)
		assert.NotContains(t, crb.Annotations, crbAdminGlobalRoleMissingAnnotation)
	})

	t.Run("group admin", func(t *testing.T) {
		crb, err := crbCleaner.sync("globaladmin-u-qbkn4w4tqr", &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "globaladmin-u-qbkn4w4tqr",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     rbac.ClusterAdminRoleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "Group",
					Name: "okta_group://admins",
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, crb)
		assert.Contains(t, crb.Annotations, rbac.CrbAdminGlobalRoleCheckedAnnotation)
		assert.NotContains(t, crb.Annotations, crbAdminGlobalRoleMissingAnnotation)
	})

	t.Run("orphaned CRB", func(t *testing.T) {
		crb, err := crbCleaner.sync("globaladmin-u-8ztgw", &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "globaladmin-u-8ztgw",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     rbac.ClusterAdminRoleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "u-8ztgw", // There are no admin GlobalRoleBindings for this user.
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, crb)
		assert.Contains(t, crb.Annotations, rbac.CrbAdminGlobalRoleCheckedAnnotation)
		assert.Contains(t, crb.Annotations, crbAdminGlobalRoleMissingAnnotation)
	})
}

func TestCRBSyncDoesntChecksGlobalAdminRoleWithAnnotation(t *testing.T) {
	ctrl := gomock.NewController(t)

	grCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.GlobalRole](ctrl)
	grCache.EXPECT().List(gomock.Any()).Times(0)

	grbCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().List(gomock.Any()).Times(0)

	crbs := fake.NewMockNonNamespacedClientInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
	crbs.EXPECT().Update(gomock.Any()).Times(0)

	crbCleaner := &crbCleaner{
		noRemainingOwnerLabels: func(*rbacv1.ClusterRoleBinding) (bool, error) { return false, nil },
		crbs:                   crbs,
		grCache:                grCache,
		grbCache:               grbCache,
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "globaladmin-u-8ztgw",
			Annotations: map[string]string{},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     rbac.ClusterAdminRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "u-8ztgw",
			},
		},
	}

	t.Run("checked and missing", func(t *testing.T) {
		crb := crb.DeepCopy()
		crb.Annotations = map[string]string{
			rbac.CrbAdminGlobalRoleCheckedAnnotation: "true",
			crbAdminGlobalRoleMissingAnnotation:      "true",
		}

		obj, err := crbCleaner.sync("globaladmin-u-8ztgw", crb)
		require.NoError(t, err)
		require.NotNil(t, obj)
	})

	t.Run("checked not missing", func(t *testing.T) {
		crb := crb.DeepCopy()
		crb.Annotations = map[string]string{
			rbac.CrbAdminGlobalRoleCheckedAnnotation: "true",
		}

		obj, err := crbCleaner.sync("globaladmin-u-8ztgw", crb)
		require.NoError(t, err)
		require.NotNil(t, obj)
	})
}
