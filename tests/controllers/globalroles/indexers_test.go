package globalroles_integration_test

import (
	"context"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementauth "github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/features"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
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
	restCfg         *rest.Config
	wranglerContext *wrangler.Context
}

func (s *IndexerTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())

	s.testEnv = &envtest.Environment{}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)
	s.restCfg = restCfg

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

	// The last result is kept so a missing indexer reports its own error rather than only a timeout.
	grbCache := s.wranglerContext.Mgmt.GlobalRoleBinding().Cache()
	var indexed []*v3.GlobalRoleBinding
	assert.Eventually(t, func() bool {
		indexed, err = grbCache.GetByIndex(pkgrbac.GRBGlobalRoleIndex, gr.Name)
		return err == nil && len(indexed) == 1
	}, duration, tick)
	assert.NoError(t, err)
	if assert.Len(t, indexed, 1) {
		assert.Equal(t, grb.Name, indexed[0].Name)
	}

	err = s.wranglerContext.Mgmt.GlobalRoleBinding().Delete(grb.Name, &metav1.DeleteOptions{})
	assert.NoError(t, err)
	err = s.wranglerContext.Mgmt.GlobalRole().Delete(gr.Name, &metav1.DeleteOptions{})
	assert.NoError(t, err)
}

// TestGRBByGlobalRoleIndexWithoutMCM covers the rancher instance embedded in the cluster agent, which
// runs the same registration with MCM off. The GlobalRoleBinding CRD is not installed there, so the
// cache must be left alone rather than started against a resource that does not exist.
func (s *IndexerTestSuite) TestGRBByGlobalRoleIndexWithoutMCM() {
	t := s.T()

	features.MCM.Set(false)
	defer features.MCM.Set(true)

	// A separate context so the indexer registered by the suite is not visible here.
	noMCMContext, err := wrangler.NewContext(s.ctx, nil, s.restCfg)
	assert.NoError(t, err)

	managementauth.RegisterWranglerIndexers(noMCMContext)

	_, err = noMCMContext.Mgmt.GlobalRoleBinding().Cache().GetByIndex(pkgrbac.GRBGlobalRoleIndex, "indexed-gr")
	assert.ErrorContains(t, err, pkgrbac.GRBGlobalRoleIndex)
}

func TestIndexerTestSuite(t *testing.T) {
	suite.Run(t, new(IndexerTestSuite))
}
