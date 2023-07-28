package fleetcharts

import (
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/settings"
	ctrlfake "github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errUnimplemented  = fmt.Errorf("unimplemented")
	errNotFound       = fmt.Errorf("not found")
	priorityClassName = "rancher-critical"
)

// Test_ChartInstallation test that all expected charts are installed or uninstalled with expected configuration
func Test_ChartInstallation(t *testing.T) {
	if oldVal, ok := os.LookupEnv("HTTP_PROXY"); ok {
		os.Unsetenv("HTTP_PROXY")
		defer os.Setenv("HTTP_PROXY", oldVal)
	}
	if oldVal, ok := os.LookupEnv("NO_PROXY"); ok {
		os.Unsetenv("NO_PROXY")
		defer os.Setenv("NO_PROXY", oldVal)
	}
	setting := &v3.Setting{
		ObjectMeta: metav1.ObjectMeta{
			Name: settings.ServerURL.Name,
		},
	}
	tests := []struct {
		name       string
		newManager func(*gomock.Controller) chart.Manager
		wantErr    bool
	}{
		{
			name: "normal installation",
			newManager: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("pass")
				manager := fake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"apiServerURL": settings.ServerURL.Get(),
					"apiServerCA":  settings.CACerts.Get(),
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
					"bootstrap": map[string]interface{}{
						"enabled":        false,
						"agentNamespace": fleetconst.ReleaseLocalNamespace,
					},
					"gitops": map[string]interface{}{
						"enabled": features.Gitops.Enabled(),
					},
					"gitjob": map[string]interface{}{
						"priorityClassName": priorityClassName,
					},
					"priorityClassName": priorityClassName,
				}
				var b bool
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.CRDChartName,
					settings.FleetMinVersion.Get(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.ChartName,
					settings.FleetMinVersion.Get(),
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				manager.EXPECT().Uninstall(fleetconst.ReleaseLegacyNamespace, fleetconst.ChartName).Return(nil)
				return manager
			},
		},
		{
			name: "installation without webhook priority class",
			newManager: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("fail")
				manager := fake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"apiServerURL": settings.ServerURL.Get(),
					"apiServerCA":  settings.CACerts.Get(),
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
					"bootstrap": map[string]interface{}{
						"enabled":        false,
						"agentNamespace": fleetconst.ReleaseLocalNamespace,
					},
					"gitops": map[string]interface{}{
						"enabled": features.Gitops.Enabled(),
					},
				}
				var b bool
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.CRDChartName,
					settings.FleetMinVersion.Get(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.ChartName,
					settings.FleetMinVersion.Get(),
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil)

				manager.EXPECT().Uninstall(fleetconst.ReleaseLegacyNamespace, fleetconst.ChartName).Return(nil)
				return manager
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			configCache := ctrlfake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
			configCache.EXPECT().Get(gomock.Any(), "pass").Return(&v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil).AnyTimes()
			configCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("not found")).AnyTimes()
			h := &handler{
				chartsConfig: chart.RancherConfigGetter{ConfigCache: configCache},
			}
			h.manager = tt.newManager(ctrl)
			_, err := h.onSetting("", setting)
			if tt.wantErr {
				assert.Error(t, err, "handler.onRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NoError(t, err, "unexpected error")
		})
	}
}
