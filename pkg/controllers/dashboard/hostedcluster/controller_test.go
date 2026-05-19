package hostedcluster

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	chartsfake "github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	priorityClassName = "rancher-critical"
	errTest           = fmt.Errorf("test error")
	errNotFound       = apierrors.NewNotFound(schema.GroupResource{}, "")
)

func Test_handler_onClusterChange(t *testing.T) {

	tests := []struct {
		name       string
		cluster    *v3.Cluster
		newManager func(t *testing.T, ctrl *gomock.Controller) chart.Manager
		wantErr    bool
	}{
		{
			name:    "nil cluster returns early",
			cluster: nil,
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				return chartsfake.NewMockManager(ctrl)
			},
		},
		{
			name: "skip installation when SkipHostedClusterChartInstallation is true",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				t.Setenv("CATTLE_SKIP_HOSTED_CLUSTER_CHART_INSTALLATION", "true")
				settings.SkipHostedClusterChartInstallation.Set("true")
				t.Cleanup(func() { settings.SkipHostedClusterChartInstallation.Set("") })
				return chartsfake.NewMockManager(ctrl)
			},
		},
		{
			name: "normal installation",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				initialAksOperatorVersion := settings.AksOperatorVersion.Get()
				t.Cleanup(func() {
					settings.ConfigMapName.Set(initialConfigMapName)
					settings.AksOperatorVersion.Set(initialAksOperatorVersion)
				})
				settings.ConfigMapName.Set("pass")
				settings.AksOperatorVersion.Set("")

				manager := chartsfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
							"imagePullSecrets":      ([]string)(nil),
						},
					},
					"httpProxy":            os.Getenv("HTTP_PROXY"),
					"httpsProxy":           os.Getenv("HTTPS_PROXY"),
					"noProxy":              os.Getenv("NO_PROXY"),
					"additionalTrustedCAs": false,
					"priorityClassName":    priorityClassName,
				}
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace,
					AksCrdChart.ReleaseName,
					AksCrdChart.ChartName,
					settings.AksOperatorVersion.Get(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace,
					AksChart.ReleaseName,
					AksChart.ChartName,
					settings.AksOperatorVersion.Get(),
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				return manager
			},
		},
		{
			name: "no priority class installation",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				initialAksOperatorVersion := settings.AksOperatorVersion.Get()
				t.Cleanup(func() {
					settings.ConfigMapName.Set(initialConfigMapName)
					settings.AksOperatorVersion.Set(initialAksOperatorVersion)
				})
				settings.ConfigMapName.Set("error")
				settings.AksOperatorVersion.Set("")
				manager := chartsfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
							"imagePullSecrets":      ([]string)(nil),
						},
					},
					"httpProxy":            os.Getenv("HTTP_PROXY"),
					"httpsProxy":           os.Getenv("HTTPS_PROXY"),
					"noProxy":              os.Getenv("NO_PROXY"),
					"additionalTrustedCAs": false,
				}
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace,
					AksCrdChart.ReleaseName,
					AksCrdChart.ChartName,
					settings.AksOperatorVersion.Get(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace,
					AksChart.ReleaseName,
					AksChart.ChartName,
					settings.AksOperatorVersion.Get(),
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				return manager
			},
		},
		{
			name: "normal installation with chart version precedence",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				initialAksOperatorVersion := settings.AksOperatorVersion.Get()
				t.Cleanup(func() {
					settings.ConfigMapName.Set(initialConfigMapName)
					settings.AksOperatorVersion.Set(initialAksOperatorVersion)
				})
				settings.ConfigMapName.Set("pass")
				manager := chartsfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
							"imagePullSecrets":      ([]string)(nil),
						},
					},
					"httpProxy":            os.Getenv("HTTP_PROXY"),
					"httpsProxy":           os.Getenv("HTTPS_PROXY"),
					"noProxy":              os.Getenv("NO_PROXY"),
					"additionalTrustedCAs": false,
					"priorityClassName":    priorityClassName,
				}

				exactVersion := "1.4.0"
				settings.AksOperatorVersion.Set(exactVersion)

				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace,
					AksCrdChart.ReleaseName,
					AksCrdChart.ChartName,
					exactVersion,
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace,
					AksChart.ReleaseName,
					AksChart.ChartName,
					exactVersion,
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				return manager
			},
		},
		{
			name: "installation with global pull secrets",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				initialAksOperatorVersion := settings.AksOperatorVersion.Get()
				t.Cleanup(func() {
					settings.ConfigMapName.Set(initialConfigMapName)
					settings.AksOperatorVersion.Set(initialAksOperatorVersion)
				})
				settings.ConfigMapName.Set("pass")
				settings.AksOperatorVersion.Set("")
				settings.SystemDefaultRegistry.Set("registry.example.com")
				settings.SystemDefaultRegistryPullSecrets.Set("pull-secret-a,pull-secret-b")

				manager := chartsfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "registry.example.com",
							"imagePullSecrets":      []string{"pull-secret-a", "pull-secret-b"},
						},
					},
					"httpProxy":            os.Getenv("HTTP_PROXY"),
					"httpsProxy":           os.Getenv("HTTPS_PROXY"),
					"noProxy":              os.Getenv("NO_PROXY"),
					"additionalTrustedCAs": false,
					"priorityClassName":    priorityClassName,
				}
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace,
					AksCrdChart.ReleaseName,
					AksCrdChart.ChartName,
					settings.AksOperatorVersion.Get(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace,
					AksChart.ReleaseName,
					AksChart.ChartName,
					settings.AksOperatorVersion.Get(),
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				return manager
			},
		},
		{
			name: "cluster with no provider config does nothing",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "plain-cluster"},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				return chartsfake.NewMockManager(ctrl) // no Ensure calls expected
			},
		},
		{
			name: "crd chart ensure failure returns error",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				initialAksOperatorVersion := settings.AksOperatorVersion.Get()
				t.Cleanup(func() {
					settings.ConfigMapName.Set(initialConfigMapName)
					settings.AksOperatorVersion.Set(initialAksOperatorVersion)
				})
				settings.ConfigMapName.Set("pass")
				settings.AksOperatorVersion.Set("")

				manager := chartsfake.NewMockManager(ctrl)
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace,
					AksCrdChart.ReleaseName,
					AksCrdChart.ChartName,
					gomock.Any(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(errTest)

				return manager
			},
			wantErr: true,
		},
		{
			name: "operator chart ensure failure returns error",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					AKSConfig: &aksv1.AKSClusterConfigSpec{},
				},
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				initialAksOperatorVersion := settings.AksOperatorVersion.Get()
				t.Cleanup(func() {
					settings.ConfigMapName.Set(initialConfigMapName)
					settings.AksOperatorVersion.Set(initialAksOperatorVersion)
				})
				settings.ConfigMapName.Set("pass")
				settings.AksOperatorVersion.Set("")

				manager := chartsfake.NewMockManager(ctrl)
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace,
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace,
					AksChart.ReleaseName,
					AksChart.ChartName,
					gomock.Any(),
					"",
					gomock.Any(),
					gomock.AssignableToTypeOf(b),
					"",
				).Return(errTest)

				return manager
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialSDR := settings.SystemDefaultRegistry.Get()
			initialSDRPull := settings.SystemDefaultRegistryPullSecrets.Get()
			t.Cleanup(func() {
				settings.SystemDefaultRegistry.Set(initialSDR)
				settings.SystemDefaultRegistryPullSecrets.Set(initialSDRPull)
			})
			ctrl := gomock.NewController(t)
			h := newHandler(ctrl)
			h.manager = tt.newManager(t, ctrl)
			got, err := h.onClusterChange("", tt.cluster)
			if tt.wantErr {
				assert.Error(t, err, "handler.onRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, err, "unexpected error")
			if !reflect.DeepEqual(got, tt.cluster) {
				t.Errorf("handler.onClusterChange() = %v, want %v", got, tt.cluster)
			}
		})
	}
}

func Test_handler_onSettingsChange(t *testing.T) {
	installedApp := &catalogv1.App{ObjectMeta: metav1.ObjectMeta{Name: "operator", Namespace: "cattle-system"}}

	tests := []struct {
		name       string
		setting    *v3.Setting
		setupApps  func(*gomock.Controller) catalogcontrollers.AppController
		newManager func(t *testing.T, ctrl *gomock.Controller) chart.Manager
		wantErr    bool
	}{
		{
			name:    "nil setting returns early",
			setting: nil,
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				return fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				return chartsfake.NewMockManager(ctrl)
			},
		},
		{
			name:    "unrelated setting name returns early",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: "some-other-setting"}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				return fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				return chartsfake.NewMockManager(ctrl)
			},
		},
		{
			name:    "no operators installed, no charts redeployed",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				appsCtrl := fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
				appsCache := fake.NewMockCacheInterface[*catalogv1.App](ctrl)
				appsCtrl.EXPECT().Cache().Return(appsCache).AnyTimes()
				appsCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errNotFound).AnyTimes()
				return appsCtrl
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				return chartsfake.NewMockManager(ctrl) // no Ensure calls expected
			},
		},
		{
			name:    "app cache error returns error",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				appsCtrl := fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
				appsCache := fake.NewMockCacheInterface[*catalogv1.App](ctrl)
				appsCtrl.EXPECT().Cache().Return(appsCache).AnyTimes()
				// One provider's Get returns a non-NotFound error; others return NotFound.
				appsCache.EXPECT().Get("cattle-system", "rancher-aks-operator").Return(nil, errTest).AnyTimes()
				appsCache.EXPECT().Get("cattle-system", "rancher-eks-operator").Return(nil, errNotFound).AnyTimes()
				appsCache.EXPECT().Get("cattle-system", "rancher-gke-operator").Return(nil, errNotFound).AnyTimes()
				return appsCtrl
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				return chartsfake.NewMockManager(ctrl)
			},
			wantErr: true,
		},
		{
			name:    "aks operator installed, SystemDefaultRegistry change triggers redeploy",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				appsCtrl := fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
				appsCache := fake.NewMockCacheInterface[*catalogv1.App](ctrl)
				appsCtrl.EXPECT().Cache().Return(appsCache).AnyTimes()
				appsCache.EXPECT().Get("cattle-system", "rancher-aks-operator").Return(installedApp, nil)
				appsCache.EXPECT().Get("cattle-system", "rancher-eks-operator").Return(nil, errNotFound)
				appsCache.EXPECT().Get("cattle-system", "rancher-gke-operator").Return(nil, errNotFound)
				return appsCtrl
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				t.Cleanup(func() { settings.ConfigMapName.Set(initialConfigMapName) })
				settings.ConfigMapName.Set("pass")
				manager := chartsfake.NewMockManager(ctrl)
				var b bool
				// AKS CRD chart
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace, AksCrdChart.ReleaseName, AksCrdChart.ChartName,
					gomock.Any(), "", nil, gomock.AssignableToTypeOf(b), "",
				).Return(nil)
				// AKS operator chart
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace, AksChart.ReleaseName, AksChart.ChartName,
					gomock.Any(), "", gomock.Any(), gomock.AssignableToTypeOf(b), "",
				).Return(nil)
				return manager
			},
		},
		{
			name:    "aks operator installed, SystemDefaultRegistryPullSecrets change triggers redeploy",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistryPullSecrets.Name}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				appsCtrl := fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
				appsCache := fake.NewMockCacheInterface[*catalogv1.App](ctrl)
				appsCtrl.EXPECT().Cache().Return(appsCache).AnyTimes()
				appsCache.EXPECT().Get("cattle-system", "rancher-aks-operator").Return(installedApp, nil)
				appsCache.EXPECT().Get("cattle-system", "rancher-eks-operator").Return(nil, errNotFound)
				appsCache.EXPECT().Get("cattle-system", "rancher-gke-operator").Return(nil, errNotFound)
				return appsCtrl
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				t.Cleanup(func() { settings.ConfigMapName.Set(initialConfigMapName) })
				settings.ConfigMapName.Set("pass")
				manager := chartsfake.NewMockManager(ctrl)
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace, AksCrdChart.ReleaseName, AksCrdChart.ChartName,
					gomock.Any(), "", nil, gomock.AssignableToTypeOf(b), "",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace, AksChart.ReleaseName, AksChart.ChartName,
					gomock.Any(), "", gomock.Any(), gomock.AssignableToTypeOf(b), "",
				).Return(nil)
				return manager
			},
		},
		{
			name:    "operator installed but ensureChart fails, returns error",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				appsCtrl := fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
				appsCache := fake.NewMockCacheInterface[*catalogv1.App](ctrl)
				appsCtrl.EXPECT().Cache().Return(appsCache).AnyTimes()
				// All three may try to install; make all return an installed app so any attempt will fail.
				appsCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(installedApp, nil).AnyTimes()
				return appsCtrl
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				t.Cleanup(func() { settings.ConfigMapName.Set(initialConfigMapName) })
				settings.ConfigMapName.Set("pass")
				manager := chartsfake.NewMockManager(ctrl)
				// The first Ensure call for any provider returns an error.
				manager.EXPECT().Ensure(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errTest).AnyTimes()
				return manager
			},
			wantErr: true,
		},
		{
			name:    "orphaned operator (no cluster) is still redeployed on settings change",
			setting: &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.SystemDefaultRegistry.Name}},
			setupApps: func(ctrl *gomock.Controller) catalogcontrollers.AppController {
				// App exists in the cluster even though there is no v3.Cluster object using it.
				appsCtrl := fake.NewMockControllerInterface[*catalogv1.App, *catalogv1.AppList](ctrl)
				appsCache := fake.NewMockCacheInterface[*catalogv1.App](ctrl)
				appsCtrl.EXPECT().Cache().Return(appsCache).AnyTimes()
				appsCache.EXPECT().Get("cattle-system", "rancher-aks-operator").Return(installedApp, nil)
				appsCache.EXPECT().Get("cattle-system", "rancher-eks-operator").Return(nil, errNotFound)
				appsCache.EXPECT().Get("cattle-system", "rancher-gke-operator").Return(nil, errNotFound)
				return appsCtrl
			},
			newManager: func(t *testing.T, ctrl *gomock.Controller) chart.Manager {
				initialConfigMapName := settings.ConfigMapName.Get()
				t.Cleanup(func() { settings.ConfigMapName.Set(initialConfigMapName) })
				settings.ConfigMapName.Set("pass")
				manager := chartsfake.NewMockManager(ctrl)
				var b bool
				manager.EXPECT().Ensure(
					AksCrdChart.ReleaseNamespace, AksCrdChart.ReleaseName, AksCrdChart.ChartName,
					gomock.Any(), "", nil, gomock.AssignableToTypeOf(b), "",
				).Return(nil)
				manager.EXPECT().Ensure(
					AksChart.ReleaseNamespace, AksChart.ReleaseName, AksChart.ChartName,
					gomock.Any(), "", gomock.Any(), gomock.AssignableToTypeOf(b), "",
				).Return(nil)
				return manager
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			secretsCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
			secretsCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			configCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
			configCache.EXPECT().Get(gomock.Any(), "pass").Return(&v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil).AnyTimes()
			configCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("not found")).AnyTimes()

			h := &handler{
				secretsCache: secretsCache,
				chartsConfig: chart.RancherConfigGetter{ConfigCache: configCache},
				manager:      tt.newManager(t, ctrl),
				apps:         tt.setupApps(ctrl),
			}

			result, err := h.onSettingsChange("", tt.setting)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.setting, result)
		})
	}
}

func Test_handler_onSecretChange(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		obj          *v1.Secret
		expectRemove bool
		removeNS     string
		removeName   string
	}{
		{
			name:         "non-nil secret, no remove",
			key:          "cattle-system/sh.helm.release.v1.rancher-aks-operator.v1",
			obj:          &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.rancher-aks-operator.v1", Namespace: "cattle-system"}},
			expectRemove: false,
		},
		{
			name:         "nil secret with wrong namespace, no remove",
			key:          "other-namespace/sh.helm.release.v1.rancher-aks-operator.v1",
			obj:          nil,
			expectRemove: false,
		},
		{
			name:         "nil secret in cattle-system but name has wrong segment count, no remove",
			key:          "cattle-system/sh.helm.release.rancher-aks-operator.v1",
			obj:          nil,
			expectRemove: false,
		},
		{
			name:         "nil secret, cattle-system, aks operator release name, triggers remove",
			key:          "cattle-system/sh.helm.release.v1.rancher-aks-operator.v1",
			obj:          nil,
			expectRemove: true,
			removeNS:     "cattle-system",
			removeName:   "rancher-aks-operator",
		},
		{
			name:         "nil secret, cattle-system, aks crd release name, triggers remove",
			key:          "cattle-system/sh.helm.release.v1.rancher-aks-operator-crd.v1",
			obj:          nil,
			expectRemove: true,
			removeNS:     "cattle-system",
			removeName:   "rancher-aks-operator-crd",
		},
		{
			name:         "nil secret, cattle-system, non-operator release name, no remove",
			key:          "cattle-system/sh.helm.release.v1.some-other-chart.v1",
			obj:          nil,
			expectRemove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := chartsfake.NewMockManager(ctrl)
			if tt.expectRemove {
				manager.EXPECT().Remove(tt.removeNS, tt.removeName)
			}
			h := &handler{manager: manager}

			result, err := h.onSecretChange(tt.key, tt.obj)
			assert.NoError(t, err)
			assert.Equal(t, tt.obj, result)
		})
	}
}

func newHandler(ctrl *gomock.Controller) *handler {
	secretsCache := fake.NewMockCacheInterface[*v1.Secret](ctrl)
	secretsCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	configCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
	configCache.EXPECT().Get(gomock.Any(), "pass").Return(&v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil).AnyTimes()
	configCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("not found")).AnyTimes()
	return &handler{
		secretsCache: secretsCache,
		chartsConfig: chart.RancherConfigGetter{ConfigCache: configCache},
	}
}
