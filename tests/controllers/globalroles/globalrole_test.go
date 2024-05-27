package globalroles_integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/controllers/management/auth/globalroles"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v2/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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

func (s *GlobalRoleTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())

	// Start envtest
	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	// Register CRDs
	common.RegisterCRDs(s.T(), s.ctx, restCfg,
		crd.CRD{
			SchemaObject: v3.GlobalRole{},
			NonNamespace: true,
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
	common.StartNormanControllers(s.T(), s.ctx, s.managementContext,
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
	common.StartWranglerCaches(s.T(), s.ctx, s.managementContext.Wrangler,
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
		})
}

func (s *GlobalRoleTestSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}

func (s *GlobalRoleTestSuite) TestCreateGlobalRole() {
	tests := []struct {
		name         string
		globalRole   v3.GlobalRole
		roles        []rbacv1.Role
		clusterRoles []rbacv1.ClusterRole
	}{
		{
			name: "global rule test",
			globalRole: v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"authz.management.cattle.io/cr-name": "cr-name",
					},
					Name: "test-gr",
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"get", "list"},
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			clusterRoles: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cr-name",
					},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"get", "list"},
							APIGroups: []string{""},
							Resources: []string{"pods"},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		test := test
		s.T().Run(test.name, func(t *testing.T) {
			g, err := s.managementContext.Management.GlobalRoles("").Create(&test.globalRole)
			assert.NoError(s.T(), err)
			fmt.Printf("%v\n", g)

			// Create Global Role
			assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
				gr, err := s.managementContext.Management.GlobalRoles("").Get(test.globalRole.Name, metav1.GetOptions{})
				assert.NoError(c, err)
				assert.NotNil(c, gr)
				assert.NotNil(c, gr.Status)

				// Once status is completed, all necessary backing resources should have been created
				//assert.Equal(c, globalroles.SummaryCompleted, gr.Status.Summary)
			}, duration, tick)

			// Check created Roles
			for _, r := range test.roles {
				role, err := s.managementContext.RBAC.Roles(r.Namespace).Get(r.Name, metav1.GetOptions{})
				assert.NoError(s.T(), err)
				// Assert any desired role fields
				assert.Equal(s.T(), test.globalRole.Name, role.OwnerReferences[0].Name)
			}

			// Check created Cluster Roles
			for _, cr := range test.clusterRoles {
				_, err := s.managementContext.RBAC.ClusterRoles("").Get(cr.Name, metav1.GetOptions{})
				assert.NoError(s.T(), err)
				// Assert any desired clusterRole fields
				//assert.Equal(s.T(), test.globalRole.Name, clusterRole.OwnerReferences[0].Name)
			}

			// Delete Global Role
			//assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
			//	err := s.managementContext.Management.GlobalRoles("").Delete(test.globalRole.Name, &metav1.DeleteOptions{})
			//	assert.NoError(c, err)
			//}, duration, tick)

			// Check that Roles get deleted
			//for _, r := range test.roles {
			//	_, err = s.managementContext.RBAC.Roles(r.Namespace).Get(r.Name, metav1.GetOptions{})
			//	assert.Error(s.T(), err)
			// TODO make sure the error is a "NotFound"
			//}

			// Check that Cluster Roles get deleted
			//for _, cr := range test.clusterRoles {
			//	_, err = s.managementContext.RBAC.ClusterRoles("").Get(cr.Name, metav1.GetOptions{})
			//	assert.Error(s.T(), err)
			// TODO make sure the error is a "NotFound"
			//}
		})
	}
}

func TestGlobalRoleTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRoleTestSuite))
}
