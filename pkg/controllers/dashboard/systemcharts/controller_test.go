package systemcharts

import (
	"fmt"
	"testing"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	errTest           = fmt.Errorf("test error")
	priorityClassName = "rancher-critical"
	operatorNamespace = "rancher-operator-system"
	priorityConfig    = &v1.ConfigMap{
		Data: map[string]string{
			"priorityClassName": priorityClassName,
		},
	}
	fullConfig = &v1.ConfigMap{
		Data: map[string]string{
			"priorityClassName":    priorityClassName,
			chart.WebhookChartName: testYAML,
		},
	}
	emptyConfig     = &v1.ConfigMap{}
	originalVersion = settings.RancherWebhookVersion.Get()
	originalMCM     = features.MCM.Enabled()
)

const testYAML = `---
newKey: newValue
mcm:
  enabled: false
global: ""
priorityClassName: newClass
`

type testMocks struct {
	manager       *chartfake.MockManager
	namespaceCtrl *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	configCache   *fake.MockCacheInterface[*v1.ConfigMap]
}

func (t *testMocks) Handler() *handler {
	return &handler{
		manager:      t.manager,
		namespaces:   t.namespaceCtrl,
		chartsConfig: chart.RancherConfigGetter{ConfigCache: t.configCache},
	}
}

// Test_ChartInstallation test that all expected charts are installed or uninstalled with expected configuration.
func Test_ChartInstallation(t *testing.T) {
	repo := &catalog.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
	}
	tests := []struct {
		name             string
		setup            func(testMocks)
		registryOverride string
		wantErr          bool
	}{
		{
			name: "normal installation",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(4)
				settings.RancherWebhookVersion.Set("2.0.0")
				settings.RancherProvisioningCAPIVersion.Set("2.0.0")
				expectedValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"capi":              nil,
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				expectedProvCAPIValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}

				mocks.manager.EXPECT().Ensure(
					namespace.ProvisioningCAPINamespace,
					"rancher-provisioning-capi",
					"",
					"2.0.0",
					expectedProvCAPIValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
		{
			name: "installation with config cache errors",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(nil, errTest).Times(4)
				settings.RancherWebhookVersion.Set("2.0.0")
				settings.RancherProvisioningCAPIVersion.Set("2.0.0")
				expectedValues := map[string]interface{}{
					"capi": nil,
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				expectedProvCAPIValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.ProvisioningCAPINamespace,
					"rancher-provisioning-capi",
					"",
					"2.0.0",
					expectedProvCAPIValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
		{
			name: "installation with image override",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(emptyConfig, nil).Times(4)
				settings.RancherWebhookVersion.Set("2.0.1")
				settings.RancherProvisioningCAPIVersion.Set("2.0.1")
				expectedValues := map[string]interface{}{
					"capi": nil,
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
					"image": map[string]interface{}{
						"repository": "rancher-test.io/rancher/rancher-webhook",
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.1",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				expectedProvCAPIValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
					"image": map[string]interface{}{
						"repository": "rancher-test.io/rancher/mirrored-cluster-api-controller",
					},
				}

				mocks.manager.EXPECT().Ensure(
					namespace.ProvisioningCAPINamespace,
					"rancher-provisioning-capi",
					"",
					"2.0.1",
					expectedProvCAPIValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
			registryOverride: "rancher-test.io",
		},
		{
			name: "installation with webhook values",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(fullConfig, nil).Times(4)
				settings.RancherWebhookVersion.Set("2.0.0")
				settings.RancherProvisioningCAPIVersion.Set("2.0.0")
				features.MCM.Set(true)
				expectedValues := map[string]interface{}{
					"priorityClassName": "newClass",
					"capi":              nil,
					"mcm": map[string]interface{}{
						"enabled": false,
					},
					"global": "",
					"newKey": "newValue",
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				expectedProvCAPIValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.ProvisioningCAPINamespace,
					"rancher-provisioning-capi",
					"",
					"2.0.0",
					expectedProvCAPIValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset setting to default values before each test
			settings.RancherWebhookVersion.Set(originalVersion)
			settings.RancherProvisioningCAPIVersion.Set(originalVersion)
			features.MCM.Set(originalMCM)

			ctrl := gomock.NewController(t)

			// create mocks for each test
			mocks := testMocks{
				manager:       chartfake.NewMockManager(ctrl),
				namespaceCtrl: fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
				configCache:   fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
			}

			// allow test to add expected calls to mocks and run any additional setup
			tt.setup(mocks)
			h := mocks.Handler()

			// add any registryOverrides
			h.registryOverride = tt.registryOverride
			_, err := h.onRepo("", repo)
			if (err != nil) != tt.wantErr {
				require.FailNow(t, "handler.onRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_relatedConfigMaps(t *testing.T) {
	const fooMap = "foo"
	orig := settings.ConfigMapName.Get()
	defer func() { settings.ConfigMapName.Set(orig) }()
	settings.ConfigMapName.Set(fooMap)
	tests := []struct {
		changedObj runtime.Object
		name       string
		want       []relatedresource.Key
	}{
		{
			name: "rancher-config change",
			changedObj: &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      chart.CustomValueMapName,
				Namespace: namespace.System,
			}},
			want: []relatedresource.Key{{Name: repoName, Namespace: ""}},
		},
		{
			name: "configMap from settings change",
			changedObj: &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      fooMap,
				Namespace: namespace.System,
			}},
			want: []relatedresource.Key{{Name: repoName, Namespace: ""}},
		},
		{
			name: "rancher-config changed wrong namespace",
			changedObj: &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      chart.CustomValueMapName,
				Namespace: "",
			}},
			want: nil,
		},
		{
			name: "configMap from settings change wrong namespace",
			changedObj: &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      fooMap,
				Namespace: fooMap,
			}},
			want: nil,
		},
		{
			name: "incorrect type",
			changedObj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{
				Name:      chart.CustomValueMapName,
				Namespace: namespace.System,
			}},
			want: nil,
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			got, err := relatedConfigMaps("", "", test.changedObj)
			require.NoError(t, err, "unexpected error")
			require.Equal(t, test.want, got)

		})
	}
}

func Test_relatedFeature(t *testing.T) {
	tests := []struct {
		changedObj runtime.Object
		name       string
		want       []relatedresource.Key
	}{
		{
			name: "feature changed",
			changedObj: &v3.Feature{ObjectMeta: metav1.ObjectMeta{
				Name: features.MCM.Name(),
			}},
			want: []relatedresource.Key{{Name: repoName, Namespace: ""}},
		},
		{
			name: "incorrect type",
			changedObj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{
				Name:      chart.CustomValueMapName,
				Namespace: namespace.System,
			}},
			want: nil,
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			got, err := relatedFeatures("", "", test.changedObj)
			require.NoError(t, err, "unexpected error")
			require.Equal(t, test.want, got)

		})
	}
}

func Test_relatedSettings(t *testing.T) {
	tests := []struct {
		changedObj runtime.Object
		name       string
		want       []relatedresource.Key
	}{
		{
			name: "rancher version",
			changedObj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{
				Name: settings.RancherWebhookVersion.Name,
			}},
			want: []relatedresource.Key{{Name: repoName, Namespace: ""}},
		},
		{
			name: "system default registry",
			changedObj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{
				Name: settings.SystemDefaultRegistry.Name,
			}},
			want: []relatedresource.Key{{Name: repoName, Namespace: ""}},
		},
		{
			name: "shell image",
			changedObj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{
				Name: settings.ShellImage.Name,
			}},
			want: []relatedresource.Key{{Name: repoName, Namespace: ""}},
		},
		{
			name: "incorrect type",
			changedObj: &v3.Feature{ObjectMeta: metav1.ObjectMeta{
				Name:      chart.CustomValueMapName,
				Namespace: namespace.System,
			}},
			want: nil,
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			got, err := relatedSettings("", "", test.changedObj)
			require.NoError(t, err, "unexpected error")
			require.Equal(t, test.want, got)

		})
	}
}
