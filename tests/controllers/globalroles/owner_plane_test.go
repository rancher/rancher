package globalroles_integration_test

import (
	"context"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementauth "github.com/rancher/rancher/pkg/controllers/management/auth"
	userrbac "github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/multiclustermanager"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
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

const grbOwnerLabel = "authz.management.cattle.io/grb-owner"

// OwnerPlaneTestSuite models a replica that owns a downstream cluster but is not the leader: only
// the registration path every replica runs is exercised, deliberately without globalroles.Register.
// The envtest cluster stands in for the downstream cluster. A Role and RoleBinding defined by a
// GlobalRole's InheritedNamespacedRules must be created when the namespace appears after the
// GlobalRole, without any leader-side controller running. See rancher/rancher#56890.
type OwnerPlaneTestSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	wranglerContext *wrangler.Context
}

func (s *OwnerPlaneTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())

	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	common.RegisterCRDs(s.ctx, s.T(), restCfg,
		crd.CRD{
			SchemaObject: v3.GlobalRole{},
			NonNamespace: true,
			Status:       true,
		},
		crd.CRD{
			SchemaObject: v3.GlobalRoleBinding{},
			NonNamespace: true,
		},
	)

	s.wranglerContext, err = wrangler.NewContext(s.ctx, nil, restCfg)
	assert.NoError(s.T(), err)

	// The registrations every replica performs at startup, and nothing else. The scaled context
	// shares the wrangler controller factory like in production, so the indexers registered through
	// the norman informers land on the same caches the handler reads.
	scaledContext, _, _, err := multiclustermanager.BuildScaledContext(s.ctx, s.wranglerContext, &multiclustermanager.Options{})
	assert.NoError(s.T(), err)
	managementauth.RegisterWranglerIndexers(s.wranglerContext)
	assert.NoError(s.T(), managementauth.RegisterIndexers(scaledContext))

	// The owner-plane handler under test, wired exactly like the user context does it.
	userrbac.RegisterInheritedNamespacedRulesHandler(s.ctx, s.wranglerContext.Core.Namespace(),
		s.wranglerContext.Mgmt.GlobalRole(), s.wranglerContext.Mgmt.GlobalRoleBinding(),
		s.wranglerContext.RBAC.Role(), s.wranglerContext.RBAC.RoleBinding(), "c-m-owner-plane")

	common.StartWranglerControllers(s.ctx, s.T(), s.wranglerContext,
		schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Namespace",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "GlobalRole",
		},
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "GlobalRoleBinding",
		},
	)

	common.StartWranglerCaches(s.ctx, s.T(), s.wranglerContext,
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
	)
}

func (s *OwnerPlaneTestSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}

func (s *OwnerPlaneTestSuite) TestRoleAndRoleBindingCreatedForLateNamespace() {
	t := s.T()

	nsName := "owner-plane-ns"
	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owner-plane-gr",
		},
		InheritedNamespacedRules: map[string][]rbacv1.PolicyRule{
			nsName: {getPodRule},
		},
	}
	_, err := s.wranglerContext.Mgmt.GlobalRole().Create(gr)
	assert.NoError(t, err)

	grb := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owner-plane-grb",
		},
		GlobalRoleName: gr.Name,
		UserName:       "u-owner-plane",
	}
	_, err = s.wranglerContext.Mgmt.GlobalRoleBinding().Create(grb)
	assert.NoError(t, err)

	// Wait until both objects are visible through the indexed caches the handler reads, so the
	// namespace event below cannot fire against a cache that has not caught up yet.
	grCache := s.wranglerContext.Mgmt.GlobalRole().Cache()
	grbCache := s.wranglerContext.Mgmt.GlobalRoleBinding().Cache()
	assert.Eventually(t, func() bool {
		grs, err := grCache.GetByIndex(pkgrbac.GRDownstreamNSIndex, nsName)
		if err != nil || len(grs) != 1 {
			return false
		}
		grbs, err := grbCache.GetByIndex(pkgrbac.GRBGlobalRoleIndex, gr.Name)
		return err == nil && len(grbs) == 1
	}, duration, tick)

	// The namespace is created after the GlobalRole, the scenario from rancher/rancher#56890.
	_, err = s.wranglerContext.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: nsName},
	})
	assert.NoError(t, err)

	roleName := gr.Name + "-" + nsName
	var role *rbacv1.Role
	assert.Eventually(t, func() bool {
		role, err = s.wranglerContext.RBAC.Role().Get(nsName, roleName, metav1.GetOptions{})
		return err == nil
	}, duration, tick, "Role was not created for the namespace added after the GlobalRole")
	if role != nil {
		assert.Equal(t, gr.Name, role.Labels[grOwnerLabel])
		assert.Equal(t, []rbacv1.PolicyRule{getPodRule}, role.Rules)
	}

	rbName := grb.Name + "-" + nsName
	var rb *rbacv1.RoleBinding
	assert.Eventually(t, func() bool {
		rb, err = s.wranglerContext.RBAC.RoleBinding().Get(nsName, rbName, metav1.GetOptions{})
		return err == nil
	}, duration, tick, "RoleBinding was not created for the namespace added after the GlobalRole")
	if rb != nil {
		assert.Equal(t, grb.Name, rb.Labels[grbOwnerLabel])
		assert.Equal(t, []rbacv1.Subject{{
			Kind:     "User",
			Name:     grb.UserName,
			APIGroup: rbacv1.GroupName,
		}}, rb.Subjects)
		assert.Equal(t, rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		}, rb.RoleRef)
	}
}

func TestOwnerPlaneTestSuite(t *testing.T) {
	suite.Run(t, new(OwnerPlaneTestSuite))
}

// TestRoleCreatedWhenGlobalRoleArrivesAfterNamespace covers the reverse ordering: the namespace
// already exists when the GlobalRole and GlobalRoleBinding are created. The GlobalRole and
// GlobalRoleBinding watches must enqueue the namespace, closing the race where a namespace event
// fires before the management caches have observed the grant objects.
func (s *OwnerPlaneTestSuite) TestRoleCreatedWhenGlobalRoleArrivesAfterNamespace() {
	t := s.T()

	nsName := "owner-plane-ns-first"
	_, err := s.wranglerContext.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: nsName},
	})
	assert.NoError(t, err)

	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owner-plane-gr-late",
		},
		InheritedNamespacedRules: map[string][]rbacv1.PolicyRule{
			nsName: {getPodRule},
		},
	}
	_, err = s.wranglerContext.Mgmt.GlobalRole().Create(gr)
	assert.NoError(t, err)

	grb := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owner-plane-grb-late",
		},
		GlobalRoleName: gr.Name,
		UserName:       "u-owner-plane-late",
	}
	_, err = s.wranglerContext.Mgmt.GlobalRoleBinding().Create(grb)
	assert.NoError(t, err)

	roleName := gr.Name + "-" + nsName
	assert.Eventually(t, func() bool {
		_, err := s.wranglerContext.RBAC.Role().Get(nsName, roleName, metav1.GetOptions{})
		return err == nil
	}, duration, tick, "Role was not created for a GlobalRole added after the namespace")

	rbName := grb.Name + "-" + nsName
	assert.Eventually(t, func() bool {
		_, err := s.wranglerContext.RBAC.RoleBinding().Get(nsName, rbName, metav1.GetOptions{})
		return err == nil
	}, duration, tick, "RoleBinding was not created for a GlobalRoleBinding added after the namespace")
}
