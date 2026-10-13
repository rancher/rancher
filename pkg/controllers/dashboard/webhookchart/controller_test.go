package webhookchart

import (
	"errors"
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	chartfake "github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	repo = &catalog.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{Name: repoName},
	}

	priorityClassName = "rancher-critical"
	priorityConfig    = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: chart.CustomValueMapName, Namespace: namespace.System},
		Data:       map[string]string{"priorityClassName": priorityClassName},
	}

	localCluster = &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "local"},
	}

	originalWebhookVersion        = settings.RancherWebhookVersion.Get()
	originalSystemDefaultRegistry = settings.SystemDefaultRegistry.Get()
	originalMCM                   = features.MCM.Enabled()
	originalMCMAgent              = features.MCMAgent.Enabled()
)

type testMocks struct {
	manager      *chartfake.MockManager
	configCache  *fake.MockCacheInterface[*v1.ConfigMap]
	clusterCache *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
	clusters     *fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList]
	clusterRepo  *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList]
}

func (m *testMocks) Handler() *handler {
	return &handler{
		manager:      m.manager,
		chartsConfig: chart.RancherConfigGetter{ConfigCache: m.configCache},
		clusterCache: m.clusterCache,
		clusters:     m.clusters,
		clusterRepo:  m.clusterRepo,
	}
}

func newTestMocks(t *testing.T) testMocks {
	t.Helper()
	_ = settings.RancherWebhookVersion.Set(originalWebhookVersion)
	_ = settings.SystemDefaultRegistry.Set(originalSystemDefaultRegistry)
	features.MCM.Set(originalMCM)
	features.MCMAgent.Set(originalMCMAgent)
	t.Cleanup(func() {
		_ = settings.RancherWebhookVersion.Set(originalWebhookVersion)
		_ = settings.SystemDefaultRegistry.Set(originalSystemDefaultRegistry)
		features.MCM.Set(originalMCM)
		features.MCMAgent.Set(originalMCMAgent)
	})

	ctrl := gomock.NewController(t)
	return testMocks{
		manager:      chartfake.NewMockManager(ctrl),
		configCache:  fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
		clusterCache: fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
		clusters:     fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl),
		clusterRepo:  fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl),
	}
}

func TestOnRepo_IgnoresNonMatchingRepo(t *testing.T) {
	mocks := newTestMocks(t)
	h := mocks.Handler()

	other := &catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{Name: "some-other-repo"}}
	// No expectations set on manager/configCache/clusterCache/clusters — any call fails the test.
	_, err := h.onRepo("", other)
	require.NoError(t, err)
}

func TestOnRepo_IgnoresNilRepo(t *testing.T) {
	mocks := newTestMocks(t)
	h := mocks.Handler()

	_, err := h.onRepo("", nil)
	require.NoError(t, err)
}

func TestOnRepo_BasicInstall(t *testing.T) {
	mocks := newTestMocks(t)
	_ = settings.RancherWebhookVersion.Set("110.0.0+up0.11.0")
	_ = settings.SystemDefaultRegistry.Set("")
	features.MCM.Set(true)
	features.MCMAgent.Set(false)

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(priorityConfig, nil).AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(localCluster, nil).Times(2)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).Return(localCluster, nil).Times(1)

	expected := map[string]interface{}{
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{"systemDefaultRegistry": ""},
		},
		"capi":                nil,
		"mcm":                 map[string]interface{}{"enabled": true},
		"priorityClassName":   priorityClassName,
		"replicaCount":        1,
		"tolerations":         []interface{}{},
		"affinity":            nil,
		"resources":           map[string]interface{}{},
		"podDisruptionBudget": map[string]interface{}{"enabled": false},
	}
	mocks.manager.EXPECT().Ensure(
		namespace.System,
		chart.WebhookChartName,
		chart.WebhookChartName,
		"",
		"110.0.0+up0.11.0",
		expected,
		true,
		"",
	).Return(nil)

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnRepo_RegistryOverride(t *testing.T) {
	mocks := newTestMocks(t)
	_ = settings.SystemDefaultRegistry.Set("registry.example.com")
	features.MCM.Set(false)
	features.MCMAgent.Set(false)

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, chart.CustomValueMapName)).
		AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(localCluster, nil).Times(2)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).Return(localCluster, nil).Times(1)

	mocks.manager.EXPECT().Ensure(
		namespace.System,
		chart.WebhookChartName,
		chart.WebhookChartName,
		"",
		gomock.Any(),
		gomock.Any(),
		true,
		"my.registry.io/"+settings.ShellImage.Get(),
	).DoAndReturn(func(_, _, _, _, _ string, values map[string]interface{}, _ bool, _ string) error {
		global, ok := values["global"].(map[string]interface{})
		require.True(t, ok, "expected global values map")
		cattle, ok := global["cattle"].(map[string]interface{})
		require.True(t, ok, "expected global.cattle values map")
		require.Equal(t, "", cattle["systemDefaultRegistry"],
			"systemDefaultRegistry should be blanked to avoid double-prefixing when registryOverride is set")

		image, ok := values["image"].(map[string]interface{})
		require.True(t, ok, "expected image values map")
		require.Equal(t, "my.registry.io/rancher/rancher-webhook", image["repository"])
		return nil
	})

	h := mocks.Handler()
	h.registryOverride = "my.registry.io"
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnRepo_WebhookDeploymentCustomizationApplied(t *testing.T) {
	mocks := newTestMocks(t)
	features.MCM.Set(true)
	features.MCMAgent.Set(false)

	replicas := int32(3)
	customized := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "local"},
		Spec: v3.ClusterSpec{
			ClusterSpecBase: v3.ClusterSpecBase{
				WebhookDeploymentCustomization: &v3.WebhookDeploymentCustomization{
					ReplicaCount: &replicas,
				},
			},
		},
	}

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, chart.CustomValueMapName)).
		AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(customized, nil).Times(2)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v3.Cluster) (*v3.Cluster, error) {
		require.NotNil(t, c.Status.AppliedWebhookDeploymentCustomization,
			"applied customization status should be populated from spec")
		require.Equal(t, replicas, *c.Status.AppliedWebhookDeploymentCustomization.ReplicaCount)
		return c, nil
	}).Times(1)

	mocks.manager.EXPECT().Ensure(
		namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", gomock.Any(), gomock.Any(), true, "",
	).DoAndReturn(func(_, _, _, _, _ string, values map[string]interface{}, _ bool, _ string) error {
		require.Equal(t, int32(3), values["replicaCount"],
			"WebhookDeploymentCustomization.ReplicaCount should override the chart default")
		return nil
	})

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnRepo_ConfigMapValuesWinOverWebhookDeploymentCustomization(t *testing.T) {
	mocks := newTestMocks(t)
	features.MCM.Set(true)
	features.MCMAgent.Set(false)

	replicas := int32(3)
	customized := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "local"},
		Spec: v3.ClusterSpec{
			ClusterSpecBase: v3.ClusterSpecBase{
				WebhookDeploymentCustomization: &v3.WebhookDeploymentCustomization{
					ReplicaCount: &replicas,
				},
			},
		},
	}
	overrideConfig := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: chart.CustomValueMapName, Namespace: namespace.System},
		Data:       map[string]string{chart.WebhookChartName: "replicaCount: 7\n"},
	}

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(overrideConfig, nil).AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(customized, nil).Times(2)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).Return(customized, nil).Times(1)

	mocks.manager.EXPECT().Ensure(
		namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", gomock.Any(), gomock.Any(), true, "",
	).DoAndReturn(func(_, _, _, _, _ string, values map[string]interface{}, _ bool, _ string) error {
		// the rancher-config ConfigMap is merged in last, so it must win over both the chart
		// default and the WebhookDeploymentCustomization value.
		require.Equal(t, float64(7), values["replicaCount"])
		return nil
	})

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnRepo_PriorityClassNotFoundIsNotFatal(t *testing.T) {
	mocks := newTestMocks(t)
	features.MCM.Set(true)
	features.MCMAgent.Set(false)

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, chart.CustomValueMapName)).
		AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(localCluster, nil).Times(2)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).Return(localCluster, nil).Times(1)

	mocks.manager.EXPECT().Ensure(
		namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", gomock.Any(), gomock.Any(), true, "",
	).DoAndReturn(func(_, _, _, _, _ string, values map[string]interface{}, _ bool, _ string) error {
		_, ok := values["priorityClassName"]
		require.False(t, ok, "priorityClassName should be absent, not zero-valued, when the ConfigMap lookup 404s")
		return nil
	})

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnRepo_MCMAgentEnabled_SkipsLocalClusterLookup(t *testing.T) {
	mocks := newTestMocks(t)
	features.MCMAgent.Set(true)

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(priorityConfig, nil).AnyTimes()
	// No clusterCache/clusters expectations: on a downstream (MCMAgent) install, webhook
	// customization comes from the rancher-config ConfigMap only, and status is
	// management-server-owned — the local cluster must not be touched at all.
	mocks.manager.EXPECT().Ensure(
		namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", gomock.Any(), gomock.Any(), true, "",
	).Return(nil)

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnRepo_EnsureErrorIsPropagated(t *testing.T) {
	mocks := newTestMocks(t)
	features.MCM.Set(true)
	features.MCMAgent.Set(false)

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(priorityConfig, nil).AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(localCluster, nil).AnyTimes()
	mocks.manager.EXPECT().Ensure(
		namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", gomock.Any(), gomock.Any(), true, "",
	).Return(errors.New("ensure failed"))

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.Error(t, err)
}

func TestOnRepo_UpdateAppliedCustomizationErrorDoesNotFailInstall(t *testing.T) {
	mocks := newTestMocks(t)
	features.MCM.Set(true)
	features.MCMAgent.Set(false)

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).
		Return(priorityConfig, nil).AnyTimes()
	mocks.clusterCache.EXPECT().Get("local").Return(localCluster, nil).Times(2)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).
		Return(nil, errors.New("boom")).Times(1)
	mocks.manager.EXPECT().Ensure(
		namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", gomock.Any(), gomock.Any(), true, "",
	).Return(nil)

	h := mocks.Handler()
	// Status update failure is logged, not returned — chart install already succeeded.
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func TestOnCluster_EnqueuesOnWebhookCustomizationDrift(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *v3.Cluster
		wantEnqueue bool
	}{
		{name: "nil cluster", cluster: nil, wantEnqueue: false},
		{
			name: "non-local cluster ignored",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "downstream-1"},
				Spec: v3.ClusterSpec{ClusterSpecBase: v3.ClusterSpecBase{
					WebhookDeploymentCustomization: &v3.WebhookDeploymentCustomization{},
				}},
			},
			wantEnqueue: false,
		},
		{
			name: "deleted local cluster ignored",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "local", DeletionTimestamp: &metav1.Time{Time: time.Now()}},
				Spec: v3.ClusterSpec{ClusterSpecBase: v3.ClusterSpecBase{
					WebhookDeploymentCustomization: &v3.WebhookDeploymentCustomization{},
				}},
			},
			wantEnqueue: false,
		},
		{
			name: "local cluster with no drift",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "local"},
			},
			wantEnqueue: false,
		},
		{
			name: "local cluster with spec/status drift",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "local"},
				Spec: v3.ClusterSpec{ClusterSpecBase: v3.ClusterSpecBase{
					WebhookDeploymentCustomization: &v3.WebhookDeploymentCustomization{},
				}},
			},
			wantEnqueue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			if tt.wantEnqueue {
				mocks.clusterRepo.EXPECT().EnqueueAfter(repoName, 2*time.Second).Times(1)
			}
			h := mocks.Handler()
			_, err := h.onCluster("", tt.cluster)
			require.NoError(t, err)
		})
	}
}

func TestRelatedSettings(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		wantKey bool
	}{
		{name: "watched setting: rancher-webhook-version", obj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.RancherWebhookVersion.Name}}, wantKey: true},
		{name: "watched setting: system-default-registry", obj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name}}, wantKey: true},
		{name: "unwatched setting", obj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: "some-other-setting"}}, wantKey: false},
		{name: "non-setting object", obj: &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}}, wantKey: false},
	}
	wantKeys := []relatedresource.Key{{Name: repoName}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := relatedSettings("", "", tt.obj)
			require.NoError(t, err)
			if tt.wantKey {
				require.Equal(t, wantKeys, keys)
			} else {
				require.Nil(t, keys)
			}
		})
	}
}

func TestRelatedFeatures(t *testing.T) {
	keys, err := relatedFeatures("", "", &v3.Feature{ObjectMeta: metav1.ObjectMeta{Name: "mcm"}})
	require.NoError(t, err)
	require.Equal(t, []relatedresource.Key{{Name: repoName}}, keys)

	keys, err = relatedFeatures("", "", &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "local"}})
	require.NoError(t, err)
	require.Nil(t, keys)
}
