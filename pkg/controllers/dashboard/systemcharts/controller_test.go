package systemcharts

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	chartfake "github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/generic/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errUnimplemented  = fmt.Errorf("unimplemented")
	errNotFound       = fmt.Errorf("not found")
	priorityClassName = "rancher-critical"
	operatorNamespace = "rancher-operator-system"
)

// Test_ChartInstallation test that all expected charts are installed or uninstalled with expected configuration
func Test_ChartInstallation(t *testing.T) {
	repo := &catalog.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
	}

	tests := []struct {
		name             string
		setup            func(*gomock.Controller) chart.Manager
		registryOverride string
		wantErr          bool
	}{
		{
			name: "normal installation",
			setup: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("pass")
				settings.RancherWebhookVersion.Set("2.0.0")
				manager := chartfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				var b bool
				manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				return manager
			},
		},
		{
			name: "installation without webhook priority class",
			setup: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("fail")
				settings.RancherWebhookVersion.Set("2.0.0")
				manager := chartfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				var b bool
				manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				return manager

			},
		},
		{
			name: "installation with image override",
			setup: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("fail")
				settings.RancherWebhookVersion.Set("2.0.1")
				manager := chartfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
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
				var b bool
				manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.1",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				return manager
			},
			registryOverride: "rancher-test.io",
		},
		{
			name: "installation with min version override",
			setup: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("fail")
				settings.RancherWebhookMinVersion.Set("2.0.1")
				settings.RancherWebhookVersion.Set("2.0.4")
				manager := chartfake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
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
				var b bool
				manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"2.0.1",
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				return manager
			},
			registryOverride: "rancher-test.io",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			namespaceCtrl := fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl)
			namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
			configCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
			configCache.EXPECT().Get(gomock.Any(), "pass").Return(&v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil).AnyTimes()
			configCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("not found")).AnyTimes()
			h := &handler{
				chartsConfig: chart.RancherConfigGetter{ConfigCache: configCache},
			}
			h.manager = tt.setup(ctrl)
			h.namespaces = namespaceCtrl
			h.registryOverride = tt.registryOverride
			_, err := h.onRepo("", repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("handler.onRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
