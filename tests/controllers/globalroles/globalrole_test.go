package globalroles_integration_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/controllers/management/auth/globalroles"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type GlobalRoleTestSuite struct {
	suite.Suite
	ctx               context.Context
	cancel            context.CancelFunc
	testEnv           *envtest.Environment
	managementContext *config.ManagementContext
}

const (
	tick     = 1 * time.Second
	duration = 20 * time.Second
)

var (
	getPodRule = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}
	readNodeRule = rbacv1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"nodes"},
	}
	editTemplates = rbacv1.PolicyRule{
		Verbs:     []string{"edit"},
		APIGroups: []string{"management.cattle.io"},
		Resources: []string{"templates"},
	}
	globalRoleLabel = "authz.management.cattle.io/globalrole"
	crNameLabel     = "authz.management.cattle.io/cr-name"
	grOwnerLabel    = "authz.management.cattle.io/gr-owner"
)

func (s *GlobalRoleTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())

	// Start envtest
	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	// Register CRDs
	common.RegisterCRDs(s.ctx, s.T(), restCfg,
		crd.CRD{
			SchemaObject: v3.GlobalRole{},
			NonNamespace: true,
			Status:       true,
		},
		crd.CRD{
			SchemaObject: v3.GlobalRoleBinding{},
			NonNamespace: true,
		})

	// Create wrangler context
	wranglerContext, err := wrangler.NewContext(s.ctx, nil, restCfg)
	assert.NoError(s.T(), err)

	// Create management context
	scaledContext, clusterManager, _, err := multiclustermanager.BuildScaledContext(s.ctx, wranglerContext, &multiclustermanager.Options{})
	assert.NoError(s.T(), err)
	s.managementContext, err = scaledContext.NewManagementContext()
	assert.NoError(s.T(), err)

	// Register controller
	globalroles.Register(s.ctx, s.managementContext, clusterManager)

	// Start controllers
	common.StartNormanControllers(s.ctx, s.T(), s.managementContext,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "GlobalRoleBinding",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "GlobalRole",
		})

	// Start caches
	common.StartWranglerCaches(s.ctx, s.T(), s.managementContext.Wrangler,
		schema.GroupVersionKind{
			Group:   "rbac.authorization.k8s.io",
			Version: "v1",
			Kind:    "ClusterRole",
		},
		schema.GroupVersionKind{
			Group:   "rbac.authorization.k8s.io",
			Version: "v1",
			Kind:    "ClusterRoleBinding",
		},
		schema.GroupVersionKind{
			Group:   "rbac.authorization.k8s.io",
			Version: "v1",
			Kind:    "Role",
		},
		schema.GroupVersionKind{
			Group:   "rbac.authorization.k8s.io",
			Version: "v1",
			Kind:    "RoleBinding",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "GlobalRole",
		},
		schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Namespace",
		})
}

func (s *GlobalRoleTestSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}

func (s *GlobalRoleTestSuite) TestCreateGlobalRole() {
	ns1 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace1",
		},
	}
	ns2 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace2",
		},
	}
	globalDataNS := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cattle-global-data",
		},
	}
	s.managementContext.Core.Namespaces("").Create(&ns1)
	s.managementContext.Core.Namespaces("").Create(&ns2)
	s.managementContext.Core.Namespaces("").Create(&globalDataNS)

	tests := []struct {
		name        string
		globalRole  v3.GlobalRole
		roles       []rbacv1.Role
		clusterRole rbacv1.ClusterRole
	}{
		// NOTE: These test can be run in parallel only if the global role names are unique
		{
			name: "create primary cluster role given cr-name",
			globalRole: v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						crNameLabel: "cr-name",
					},
					Name: "cr-name-gr",
				},
				Rules: []rbacv1.PolicyRule{getPodRule},
			},
			clusterRole: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cr-name",
				},
				Rules: []rbacv1.PolicyRule{getPodRule},
			},
		},
		{
			name: "create primary cluster role with generated name",
			globalRole: v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "generated-name-gr",
				},
				Rules: []rbacv1.PolicyRule{getPodRule},
			},
			clusterRole: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cattle-globalrole-generated-name-gr",
				},
				Rules: []rbacv1.PolicyRule{getPodRule},
			},
		},
		{
			name: "global role with catalog role",
			globalRole: v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "catalog-role-gr",
				},
				Rules: []rbacv1.PolicyRule{getPodRule, editTemplates},
			},
			clusterRole: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cattle-globalrole-catalog-role-gr",
				},
				Rules: []rbacv1.PolicyRule{getPodRule, editTemplates},
			},
			roles: []rbacv1.Role{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "catalog-role-gr-global-catalog",
						Namespace: "cattle-global-data",
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"management.cattle.io"},
							Resources: []string{"catalogtemplates"},
							Verbs:     []string{"edit"},
						},
					},
				},
			},
		},
		{
			name: "global role with namespaced rules",
			globalRole: v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespaced-rules-gr",
				},
				NamespacedRules: map[string][]rbacv1.PolicyRule{
					"namespace1": {getPodRule, readNodeRule},
					"namespace2": {getPodRule},
				},
			},
			clusterRole: rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cattle-globalrole-namespaced-rules-gr",
				},
			},
			roles: []rbacv1.Role{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "namespaced-rules-gr-namespace1",
						Namespace: "namespace1",
						Labels: map[string]string{
							grOwnerLabel: "namespaced-rules-gr",
						},
					},
					Rules: []rbacv1.PolicyRule{getPodRule, readNodeRule},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "namespaced-rules-gr-namespace2",
						Namespace: "namespace2",
						Labels: map[string]string{
							grOwnerLabel: "namespaced-rules-gr",
						},
					},
					Rules: []rbacv1.PolicyRule{getPodRule},
				},
			},
		},
	}
	for _, test := range tests {
		test := test
		s.Run(test.name, func() {
			t := s.T()
			t.Parallel()
			gr, err := s.managementContext.Management.GlobalRoles("").Create(&test.globalRole)
			assert.NoError(t, err)

			// testenv does not run any garbage collection, objects won't get deleted.
			// To ensure that deletion would work, instead we check the created resources have the right OwnerReference
			grOwnerRef := metav1.OwnerReference{
				APIVersion: "management.cattle.io/v3",
				Kind:       "GlobalRole",
				Name:       test.globalRole.Name,
				UID:        gr.UID,
			}

			// Create Global Role
			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				gr, err = s.managementContext.Management.GlobalRoles("").Get(test.globalRole.Name, metav1.GetOptions{})
				assert.NoError(c, err)
				assert.NotNil(c, gr)
				assert.NotNil(c, gr.Status)

				// Once status is completed, all necessary backing resources should have been created
				assert.Equal(c, globalroles.SummaryCompleted, gr.Status.Summary)
			}, duration, tick)

			// Check created Cluster Role
			clusterRole, err := s.managementContext.RBAC.ClusterRoles("").Get(test.clusterRole.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.True(t, reflect.DeepEqual(test.clusterRole.Rules, clusterRole.Rules))
			// Check the ownership to ensure deletion would work
			assert.Contains(t, clusterRole.OwnerReferences, grOwnerRef)
			// ClusterRole should always indicate it is from a globalrole
			assert.Contains(t, clusterRole.Labels, globalRoleLabel)
			assert.Equal(t, "true", clusterRole.Labels[globalRoleLabel])

			// Check created Roles
			for _, r := range test.roles {
				role, err := s.managementContext.RBAC.Roles(r.Namespace).Get(r.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				// Assert any desired role fields
				assert.True(t, reflect.DeepEqual(r.Rules, role.Rules))

				// Check that the owner label exists and is set correctly
				if _, ok := r.Labels[grOwnerLabel]; ok {
					owner, ok := role.Labels[grOwnerLabel]
					assert.True(t, ok)
					assert.Equal(t, r.Labels[grOwnerLabel], owner)
				}

				// Check the ownership to ensure deletion would work
				assert.Contains(t, role.OwnerReferences, grOwnerRef)
			}

			// clean up
			err = s.managementContext.Management.GlobalRoles("").Delete(test.globalRole.Name, &metav1.DeleteOptions{})
			assert.NoError(t, err)
			err = s.managementContext.RBAC.ClusterRoles("").Delete(test.clusterRole.Name, &metav1.DeleteOptions{})
			assert.NoError(t, err)
			for _, r := range test.roles {
				err = s.managementContext.RBAC.Roles(r.Namespace).Delete(r.Name, &metav1.DeleteOptions{})
				assert.NoError(t, err)
			}
		})
	}
}

func TestGlobalRoleTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRoleTestSuite))
}
