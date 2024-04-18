package feature_test

import (
	"context"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/feature"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/crd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	tick     = 1 * time.Second
	duration = 10 * time.Second
)

var (
	truePtr = &[]bool{true}[0]
)

func (s *FeatureTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.TODO())
	// Load CRD from YAML for REST Client
	s.testEnv = &envtest.Environment{}
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
	cf := s.wranglerContext.ControllerFactory.ForResourceKind(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "features",
	}, "Feature", false)
	assert.NoError(s.T(), err)
	err = cf.Start(s.ctx, 1)
	assert.NoError(s.T(), err)

	// Start the NodeDriver cache
	_, err = s.wranglerContext.ControllerFactory.SharedCacheFactory().ForKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "NodeDriver",
	})
	assert.NoError(s.T(), err)
	err = s.wranglerContext.ControllerFactory.SharedCacheFactory().StartGVK(s.ctx, schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "NodeDriver",
	})
	assert.NoError(s.T(), err)

	// The feature controller checks the harvester NodeDriver, so it needs to exist
	n, err := s.wranglerContext.Mgmt.NodeDriver().Create(&v3.NodeDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "harvester",
		},
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), n)

	// Initialize Features - this creates all feature resources used by rancher
	features.InitializeFeatures(s.wranglerContext.Mgmt.Feature(), "")
}

func (s *FeatureTestSuite) TestHarvesterFeature() {
	t := assert.New(s.T())
	// If the harvester feature is enabled, it needs to enable the NodeDriver as well
	n, err := s.wranglerContext.Mgmt.NodeDriver().Get("harvester", metav1.GetOptions{})
	t.NoError(err)
	t.NotNil(n)

	nCopy := n.DeepCopy()
	nCopy.Spec.Active = false
	_, err = s.wranglerContext.Mgmt.NodeDriver().Update(nCopy)
	t.NoError(err)

	s.triggerFeatureSync("harvester")

	t.EventuallyWithT(func(c *assert.CollectT) {
		nd, err := s.wranglerContext.Mgmt.NodeDriver().Get("harvester", metav1.GetOptions{})
		assert.NoError(c, err)
		assert.True(c, nd.Spec.Active)
	}, duration, tick, "harvester feature has not been enabled")

}

func (s *FeatureTestSuite) TestHarvesterBaremetalFeature() {
	t := assert.New(s.T())
	// Set harvester baremetal feature to true
	features.GetFeatureByName("harvester-baremetal-container-workload").Set(true)

	// Harvester feature should get set if baremetal feature is enabled
	h, err := s.wranglerContext.Mgmt.Feature().Get("harvester", metav1.GetOptions{})
	t.NoError(err)
	t.NotNil(h)

	hCopy := h.DeepCopy()
	h.Spec.Value = nil
	_, err = s.wranglerContext.Mgmt.Feature().Update(hCopy)
	t.NoError(err)

	// Baremetal feature gets annotation added if it's not there
	hf, err := s.wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", metav1.GetOptions{})
	t.NoError(err)
	t.NotNil(hf)

	harvesterFeature := hf.DeepCopy()
	harvesterFeature.Annotations = nil
	_, err = s.wranglerContext.Mgmt.Feature().Update(harvesterFeature)
	t.NoError(err)

	// Check that feature.cattle.io/experimental gets set to true by the controller
	t.EventuallyWithT(func(c *assert.CollectT) {
		f, err := s.wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", metav1.GetOptions{})
		assert.NoError(c, err)
		assert.Equal(c, v3.ExperimentalFeatureValue, f.Annotations[v3.ExperimentalFeatureKey])
	}, duration, tick, "annotation feature.cattle.io/experimental has not changed to true")

	// Check that harvester feature gets set to true
	t.EventuallyWithT(func(c *assert.CollectT) {
		nd, err := s.wranglerContext.Mgmt.Feature().Get("harvester", metav1.GetOptions{})
		assert.NoError(c, err)
		assert.Equal(c, truePtr, nd.Spec.Value)
	}, duration, tick, "harvester feature has not been enabled")
}

// triggerFeatureSync updates an annotation on the feature triggering a sync to occur
// useful when testing that a non-feature setting gets modified
func (s *FeatureTestSuite) triggerFeatureSync(name string) {
	h, _ := s.wranglerContext.Mgmt.Feature().Get(name, metav1.GetOptions{})
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
