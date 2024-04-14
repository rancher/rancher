package feature_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/feature"
	"github.com/rancher/rancher/pkg/features"
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
	harvesterFeatureNoAnnotation = v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester-baremetal-container-workload",
		},
	}
	harvesterFeature = v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester-baremetal-container-workload",
			Annotations: map[string]string{
				v3.ExperimentalFeatureValue: v3.ExperimentalFeatureKey,
			},
		},
	}
	//t = true
	//f = false
	h = v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester",
		},
	}
)

func (s *FeatureTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())
	// Load CRD from YAML for REST Client
	s.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "pkg", "crds", "yaml", "generated", "management.cattle.io_features.yaml"),
			filepath.Join("..", "..", "..", "pkg", "crds", "yaml", "generated", "management.cattle.io_nodedrivers.yaml"),
		},
		ErrorIfCRDPathMissing: true,
	}
	restCfg, err := s.testEnv.Start()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), restCfg)

	// Create CRDs
	factory, err := crd.NewFactoryFromClient(restCfg)
	assert.NoError(s.T(), err)
	err = factory.BatchCreateCRDs(s.ctx,
		crd.CRD{
			SchemaObject: v3.Feature{},
			NonNamespace: true,
		},
		crd.CRD{
			SchemaObject: v3.NodeDriver{},
			NonNamespace: true,
		},
	).BatchWait()
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

	s.wranglerContext.ControllerFactory.SharedCacheFactory().StartGVK(s.ctx, schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "nodedrivers",
	})

	// The feature controller checks the harvester NodeDriver, so it needs to exist
	n, err := s.wranglerContext.Mgmt.NodeDriver().Create(&v3.NodeDriver{
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester",
		},
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), n)

	// Initialize Features
	features.InitializeFeatures(s.wranglerContext.Mgmt.Feature(), "")
}

func (s *FeatureTestSuite) TestHarvesterFeature() {
	// If the harvester feature is enabled, it needs to enable the NodeDriver as well
	n, err := s.wranglerContext.Mgmt.NodeDriver().Get("harvester", v1.GetOptions{})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), n)

	nCopy := n.DeepCopy()
	nCopy.Spec.Active = false
	_, err = s.wranglerContext.Mgmt.NodeDriver().Update(nCopy)
	assert.NoError(s.T(), err)

	s.triggerSync("harvester")

	assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		nd, err := s.wranglerContext.Mgmt.NodeDriver().Get("harvester", v1.GetOptions{})
		assert.NoError(c, err)
		assert.True(c, nd.Spec.Active)
	}, 10*time.Second, 1*time.Second, "harvester feature has not been enabled")

}

func (s *FeatureTestSuite) TestHarvesterBaremetalFeature() {
	// Set harvester baremetal feature to true
	features.GetFeatureByName("harvester-baremetal-container-workload").Set(true)

	// Harvester feature should get set if baremetal feature is enabled
	h, err := s.wranglerContext.Mgmt.Feature().Get("harvester", v1.GetOptions{})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), h)

	hCopy := h.DeepCopy()
	h.Spec.Value = nil
	_, err = s.wranglerContext.Mgmt.Feature().Update(hCopy)
	assert.NoError(s.T(), err)

	// Baremetal feature gets annotation added if it's not there
	hf, err := s.wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", v1.GetOptions{})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), hf)

	harvesterFeature := hf.DeepCopy()
	harvesterFeature.Annotations = nil
	_, err = s.wranglerContext.Mgmt.Feature().Update(harvesterFeature)
	assert.NoError(s.T(), err)

	// Check that feature.cattle.io/experimental gets set to true by the controller
	assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		f, err := s.wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", v1.GetOptions{})
		assert.NoError(c, err)
		assert.Equal(c, v3.ExperimentalFeatureValue, f.Annotations[v3.ExperimentalFeatureKey])
	}, 10*time.Second, 1*time.Second, "annotation feature.cattle.io/experimental has not changed to true")

	// Check that harvester feature gets set
	assert.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		nd, err := s.wranglerContext.Mgmt.Feature().Get("harvester", v1.GetOptions{})
		assert.NoError(c, err)
		assert.NotNil(c, nd.Spec.Value)
	}, 10*time.Second, 1*time.Second, "harvester feature has not been enabled")

	// Once set, confirm that the harvester feature is enabled
	nd, err := s.wranglerContext.Mgmt.Feature().Get("harvester", v1.GetOptions{})
	assert.NoError(s.T(), err)
	assert.True(s.T(), *nd.Spec.Value)

}

// triggerSync updates an annotation on the feature triggering a sync to occur
// useful when testing that a non-feature setting gets modified
func (s *FeatureTestSuite) triggerSync(name string) {
	h, _ := s.wranglerContext.Mgmt.Feature().Get(name, v1.GetOptions{})
	hCopy := h.DeepCopy()
	if hCopy.Annotations == nil {
		hCopy.Annotations = map[string]string{"test": "test"}
	} else {
		hCopy.Annotations["test"] = "test"
	}
	s.wranglerContext.Mgmt.Feature().Update(hCopy)
}

func (s *FeatureTestSuite) TearDownSuite() {
	s.cancel()
	err := s.testEnv.Stop()
	assert.NoError(s.T(), err)
}

func TestFeatureTestSuite(t *testing.T) {
	suite.Run(t, new(FeatureTestSuite))
}
