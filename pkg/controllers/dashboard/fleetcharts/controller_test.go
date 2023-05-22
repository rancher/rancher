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
	corev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	h := &handler{
		chartsConfig: chart.RancherConfigGetter{ConfigCache: &mockCache{}},
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

type mockCache struct {
	Maps []*v1.ConfigMap
}

func (m *mockCache) Get(namespace string, name string) (*v1.ConfigMap, error) {
	if name != "pass" {
		return nil, errNotFound
	}
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{"priorityClassName": priorityClassName},
	}, nil
}

func (m *mockCache) List(namespace string, selector labels.Selector) ([]*v1.ConfigMap, error) {
	return nil, errUnimplemented
}

func (m *mockCache) AddIndexer(indexName string, indexer corev1.ConfigMapIndexer) {}

func (m *mockCache) GetByIndex(indexName, key string) ([]*v1.ConfigMap, error) {
	return nil, errUnimplemented
}
