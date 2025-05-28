package managedcluster

import (
	"context"
	"sync"
	"testing"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	controllerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGlobalMetadataGatherer(t *testing.T) {
	// Reset globals before each test run
	mdGatherer = nil
	initMdGathererOnce = sync.Once{}
	testMu := &sync.Mutex{}

	err := RegisterCallback(
		CallbackMetricID,
		func(_ []*apiv3.Cluster) {},
	)
	assert.Error(t, err, "should error when registering a callback before initialization")

	gomockCtrl := gomock.NewController(t)
	mockClusterCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Cluster](gomockCtrl)
	mockClusterCache.EXPECT().List(gomock.Any()).Return([]*apiv3.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: apiv3.ClusterSpec{
				DisplayName: "Test Cluster",
			},
		},
	}, nil).AnyTimes()

	NewMetadataGatherer(
		mockClusterCache,
		GatherOpts{
			CollectInterval: 1 * time.Millisecond,
			Ctx:             t.Context(),
		},
	)
	assert.NotNil(t, mdGatherer, "global gatherer should not be nil after initialization")

	inMap := map[string]*apiv3.Cluster{}
	RegisterCallback(
		CallbackMetricID,
		func(clusters []*apiv3.Cluster) {
			testMu.Lock()
			defer testMu.Unlock()
			for _, cluster := range clusters {
				inMap[cluster.Name] = cluster
			}

		},
	)

	assert.Eventually(t, func() bool {
		testMu.Lock()
		defer testMu.Unlock()
		return assert.ObjectsAreEqual(inMap, map[string]*apiv3.Cluster{
			"test-cluster": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: apiv3.ClusterSpec{
					DisplayName: "Test Cluster",
				},
			},
		})
	}, 50*time.Millisecond, 1*time.Millisecond)

	inMap2 := map[string]*apiv3.Cluster{}
	RegisterCallback(
		CallbackSccID,
		func(clusters []*apiv3.Cluster) {
			testMu.Lock()
			defer testMu.Unlock()
			for _, cluster := range clusters {
				inMap2["hello-"+cluster.Name] = cluster
			}
		},
	)

	assert.Eventually(t, func() bool {
		testMu.Lock()
		defer testMu.Unlock()
		return assert.ObjectsAreEqual(inMap2, map[string]*apiv3.Cluster{
			"hello-test-cluster": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: apiv3.ClusterSpec{
					DisplayName: "Test Cluster",
				},
			},
		})
	}, 50*time.Millisecond, 1*time.Millisecond)
	testMu.Lock()
	defer testMu.Unlock()
	assert.False(t, assert.ObjectsAreEqual(inMap, inMap2), "the two maps should not be equal, as they are modified by different callbacks")
}

type tc struct {
	opts         GatherOpts
	clusterCache controllerv3.ClusterCache

	expectedMap map[string]*apiv3.Cluster
	registerCbs map[string]func(map[string]*apiv3.Cluster) func([]*apiv3.Cluster)
	inputMap    map[string]*apiv3.Cluster
}

func newTestcase(
	opts GatherOpts,
	clusterCache controllerv3.ClusterCache,
	expectedMap map[string]*apiv3.Cluster,
	registerCbs map[string]func(map[string]*apiv3.Cluster) func([]*apiv3.Cluster),
) tc {
	return tc{
		opts:         opts,
		clusterCache: clusterCache,
		expectedMap:  expectedMap,
		registerCbs:  registerCbs,
		inputMap:     map[string]*apiv3.Cluster{},
	}
}

func TestMetadataGathererInstance(t *testing.T) {
	testMu := &sync.Mutex{}
	ctx, ca := context.WithCancel(t.Context())
	defer ca()
	gomockCtrl := gomock.NewController(t)

	mockClusterCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.Cluster](gomockCtrl)
	mockClusterCache.EXPECT().List(gomock.Any()).Return([]*apiv3.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
			Spec: apiv3.ClusterSpec{
				DisplayName: "Test Cluster",
			},
		},
	}, nil).AnyTimes()

	tcs := []tc{
		newTestcase(
			GatherOpts{
				Ctx:             ctx,
				CollectInterval: 1 * time.Millisecond,
			},
			mockClusterCache,
			map[string]*apiv3.Cluster{
				"test-cluster": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: apiv3.ClusterSpec{
						DisplayName: "Test Cluster",
					},
				},
			},
			map[string]func(map[string]*apiv3.Cluster) func([]*apiv3.Cluster){
				"cb": func(in map[string]*apiv3.Cluster) func([]*apiv3.Cluster) {
					return func(clusters []*apiv3.Cluster) {
						testMu.Lock()
						defer testMu.Unlock()
						for _, cluster := range clusters {
							in[cluster.Name] = cluster
						}
					}
				},
			},
		),
		newTestcase(
			GatherOpts{
				Ctx:             ctx,
				CollectInterval: 1 * time.Millisecond,
			},
			mockClusterCache,
			map[string]*apiv3.Cluster{
				"cb1-test-cluster": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: apiv3.ClusterSpec{
						DisplayName: "Test Cluster",
					},
				},
				"cb2-test-cluster": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
					Spec: apiv3.ClusterSpec{
						DisplayName: "Test Cluster",
					},
				},
			},
			map[string]func(map[string]*apiv3.Cluster) func([]*apiv3.Cluster){
				"cb": func(in map[string]*apiv3.Cluster) func([]*apiv3.Cluster) {
					return func(clusters []*apiv3.Cluster) {
						testMu.Lock()
						defer testMu.Unlock()
						for _, cluster := range clusters {
							in["cb1-"+cluster.Name] = cluster
						}
					}
				},
				"cb2": func(in map[string]*apiv3.Cluster) func([]*apiv3.Cluster) {
					return func(clusters []*apiv3.Cluster) {
						testMu.Lock()
						defer testMu.Unlock()
						for _, cluster := range clusters {
							in["cb2-"+cluster.Name] = cluster
						}
					}
				},
			},
		),
	}

	for _, tc := range tcs {
		mg := newMetadataGatherer(
			tc.clusterCache,
			tc.opts,
		)
		ret := map[string]*apiv3.Cluster{}

		for name, cb := range tc.registerCbs {
			mg.registerCallback(name, cb(tc.inputMap))
		}

		assert.Len(t, ret, 0, "No clusters should be collected before the first tick")

		go mg.Run()
		assert.Eventually(t, func() bool {
			testMu.Lock()
			defer testMu.Unlock()
			return assert.ObjectsAreEqual(tc.inputMap, tc.expectedMap)
		},
			50*time.Millisecond,
			1*time.Millisecond,
			"callbacks did not modify the input map as expected",
		)
	}

}
