package globalroles_integration_test

import (
	"context"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementauth "github.com/rancher/rancher/pkg/controllers/management/auth"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// IndexerTestSuite models a replica that owns a downstream cluster but is not the leader. Only the
// registration path that every replica runs is exercised, so an indexer that moves back into the
// leader-only globalroles.Register will fail here. See rancher/rancher#56890.
type IndexerTestSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	wranglerContext *wrangler.Context
}

func (s *IndexerTestSuite) SetupSuite() {
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

	// Deliberately without globalroles.Register, which only runs on the leader.
	managementauth.RegisterWranglerIndexers(s.wranglerContext)

	common.StartWranglerCaches(s.ctx, s.T(), s.wranglerContext,
		schema.GroupVersionKind{
			Group:   "management.cattle.io",
			Version: "v3",
			Kind:    "GlobalRoleBinding",
		},
	)
}

func (s *IndexerTestSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}

// TestGRBByGlobalRoleIndex covers the lookup the downstream enqueue-grbs-by-namespace handler performs
// when a namespace referenced by inheritedNamespacedRules is created after the GlobalRole.
func (s *IndexerTestSuite) TestGRBByGlobalRoleIndex() {
	t := s.T()

	gr := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "indexed-gr",
		},
	}
	_, err := s.wranglerContext.Mgmt.GlobalRole().Create(gr)
	assert.NoError(t, err)

	grb := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "indexed-grb",
		},
		GlobalRoleName: gr.Name,
		UserName:       "u-indexed",
	}
	_, err = s.wranglerContext.Mgmt.GlobalRoleBinding().Create(grb)
	assert.NoError(t, err)

	grbCache := s.wranglerContext.Mgmt.GlobalRoleBinding().Cache()
	assert.Eventually(t, func() bool {
		grbs, err := grbCache.GetByIndex(pkgrbac.GRBGlobalRoleIndex, gr.Name)
		return err == nil && len(grbs) == 1
	}, duration, tick)

	// Repeated outside Eventually so a missing indexer reports its own error instead of a timeout.
	grbs, err := grbCache.GetByIndex(pkgrbac.GRBGlobalRoleIndex, gr.Name)
	assert.NoError(t, err)
	if assert.Len(t, grbs, 1) {
		assert.Equal(t, grb.Name, grbs[0].Name)
	}

	err = s.wranglerContext.Mgmt.GlobalRoleBinding().Delete(grb.Name, &metav1.DeleteOptions{})
	assert.NoError(t, err)
	err = s.wranglerContext.Mgmt.GlobalRole().Delete(gr.Name, &metav1.DeleteOptions{})
	assert.NoError(t, err)
}

func TestIndexerTestSuite(t *testing.T) {
	suite.Run(t, new(IndexerTestSuite))
}
