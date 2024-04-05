package feature_test

import (
	"context"
	"path/filepath"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/feature"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type FeatureTestSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	wranglerContext *wrangler.Context
}

var (
	harvesterFeature = v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester-baremetal-container-workload",
		},
	}
)

func (s *FeatureTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())
	// Load CRD from YAML for REST Client
	s.testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "pkg", "crds", "yaml", "generated", "management.cattle.io_features.yaml")},
		ErrorIfCRDPathMissing: true,
	}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	// Create feature CRD
	factory, err := crd.NewFactoryFromClient(restCfg)
	assert.NoError(s.T(), err)
	err = factory.BatchCreateCRDs(s.ctx, crd.CRD{
		SchemaObject: v3.Feature{},
		NonNamespace: true,
	}).BatchWait()
	assert.NoError(s.T(), err)

	// Create the wrangler context
	s.wranglerContext, err = wrangler.NewContext(s.ctx, nil, restCfg)
	assert.NoError(s.T(), err)

	// Register the feature controller
	feature.Register(s.ctx, s.wranglerContext)

	// Create and start the feature controller factory
	sc := s.wranglerContext.ControllerFactory.ForResourceKind(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "features",
	}, "Feature", false)
	assert.NoError(s.T(), err)
	err = sc.Start(s.ctx, 1)
	assert.NoError(s.T(), err)
}

func (s *FeatureTestSuite) TestHarvesterFeature() {
	// Create harvester feature
	f1, err := s.wranglerContext.Mgmt.Feature().Create(&harvesterFeature)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), f1)

	// Check that feature.cattle.io/experimental gets set to true by the controller
	assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		f, err := wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", v1.GetOptions{})
		assert.NoError(c, err)
		assert.Equal(c, v3.ExperimentalFeatureValue, f.Annotations[v3.ExperimentalFeatureKey])
	}, 1*time.Second, 10*time.Second, "feature.cattle.io/experimental has not changed to true")

	// Remove the feature
	err = wranglerContext.Mgmt.Feature().Delete("harvester-baremetal-container-workload", &v1.DeleteOptions{})
	assert.NoError(s.T(), err)
}

func (s *FeatureTestSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}
