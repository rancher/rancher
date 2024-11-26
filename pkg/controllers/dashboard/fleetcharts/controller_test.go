package fleetcharts

import (
	"fmt"
	"os"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/settings"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	stgs := []string{
		settings.ServerURL.Name,
		settings.CACerts.Name,
		settings.SystemDefaultRegistry.Name,
		settings.FleetMinVersion.Name,
		settings.FleetVersion.Name,
	}
	tests := []struct {
		name          string
		newManager    func(*gomock.Controller) chart.Manager
		rancherConfig map[string]string
		wantErr       bool
	}{
		{
			name: "normal installation",
			newManager: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("pass")
				settings.FleetMinVersion.Set("")

				manager := fake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"agentTLSMode": settings.AgentTLSMode.Get(),
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

				exactVersion := "0.7.0"
				settings.FleetVersion.Set(exactVersion)

				var b bool
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.CRDChartName,
					exactVersion,
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.ChartName,
					exactVersion,
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))

				manager.EXPECT().Uninstall(fleetconst.ReleaseLegacyNamespace, fleetconst.ChartName).Return(nil).Times(len(stgs))
				return manager
			},
		},
		{
			name: "normal installation with min version precedence",
			newManager: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("pass")
				manager := fake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"agentTLSMode": settings.AgentTLSMode.Get(),
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

				minVersion := "0.7.0"
				exactVersion := "0.7.1"
				settings.FleetVersion.Set(exactVersion)
				settings.FleetMinVersion.Set(minVersion)

				var b bool
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.CRDChartName,
					minVersion,
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.ChartName,
					minVersion,
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))

				manager.EXPECT().Uninstall(fleetconst.ReleaseLegacyNamespace, fleetconst.ChartName).Return(nil).Times(len(stgs))
				return manager
			},
		},
		{
			name: "installation without priority class",
			newManager: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("fail")
				settings.FleetMinVersion.Set("")

				manager := fake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"agentTLSMode": settings.AgentTLSMode.Get(),
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
					settings.FleetVersion.Get(),
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.ChartName,
					settings.FleetVersion.Get(),
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))

				manager.EXPECT().Uninstall(fleetconst.ReleaseLegacyNamespace, fleetconst.ChartName).Return(nil).Times(len(stgs))
				return manager
			},
		},
		{
			name: "installation with additional values",
			rancherConfig: map[string]string{
				"fleet": `
leaderElection:
  leaseDuration: 2s
bootstrap:
  enabled: true
gitjob:
  debug: true
`},
			newManager: func(ctrl *gomock.Controller) chart.Manager {
				settings.ConfigMapName.Set("pass")
				settings.FleetMinVersion.Set("")

				manager := fake.NewMockManager(ctrl)
				expectedValues := map[string]interface{}{
					"agentTLSMode": settings.AgentTLSMode.Get(),
					"apiServerURL": settings.ServerURL.Get(),
					"apiServerCA":  settings.CACerts.Get(),
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
					"bootstrap": map[string]interface{}{
						"enabled":        true,
						"agentNamespace": fleetconst.ReleaseLocalNamespace,
					},
					"gitops": map[string]interface{}{
						"enabled": features.Gitops.Enabled(),
					},
					"gitjob": map[string]interface{}{
						"priorityClassName": priorityClassName,
						"debug":             true,
					},
					"priorityClassName": priorityClassName,
					"leaderElection": map[string]interface{}{
						"leaseDuration": "2s",
					},
				}

				exactVersion := "0.7.0"
				settings.FleetVersion.Set(exactVersion)

				var b bool
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.CRDChartName,
					exactVersion,
					"",
					nil,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))
				manager.EXPECT().Ensure(
					fleetconst.ReleaseNamespace,
					fleetconst.ChartName,
					exactVersion,
					"",
					expectedValues,
					gomock.AssignableToTypeOf(b),
					"",
				).Return(nil).Times(len(stgs))

				manager.EXPECT().Uninstall(fleetconst.ReleaseLegacyNamespace, fleetconst.ChartName).Return(nil).Times(len(stgs))
				return manager
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			configCache := ctrlfake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
			configCache.EXPECT().
				Get(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ns, name string) (*v1.ConfigMap, error) {
					if name == "pass" {
						return &v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil
					}
					if name == "rancher-config" && tt.rancherConfig != nil {
						return &v1.ConfigMap{Data: tt.rancherConfig}, nil
					}
					return nil, apierrors.NewNotFound(v1.Resource("Configmap"), name)
				}).AnyTimes()
			h := &handler{
				chartsConfig: chart.RancherConfigGetter{ConfigCache: configCache},
			}
			h.manager = tt.newManager(ctrl)
			for _, setting := range stgs {
				_, err := h.onSetting("", &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: setting}})
				if tt.wantErr {
					assert.Error(t, err, "handler.onRepo() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				assert.NoError(t, err, "unexpected error")
			}
		})
	}
}
