package systemcharts

import (
	"fmt"
	"os"
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	chartfake "github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	emptyConfig                            = &v1.ConfigMap{}
	originalWebhookVersion                 = settings.RancherWebhookVersion.Get()
	originalCAPIVersion                    = settings.RancherProvisioningCAPIVersion.Get()
	originalSUCVersion                     = settings.SystemUpgradeControllerChartVersion.Get()
	originalImportedVersionManagement      = settings.ImportedClusterVersionManagement.Get()
	originalMCM                            = features.MCM.Enabled()
	originalMCMAgent                       = features.MCMAgent.Enabled()
	originalManagedSystemUpgradeController = features.ManagedSystemUpgradeController.Enabled()
	originalTurtles                        = features.Turtles.Enabled()
	sucDeployment                          = &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      chart.SystemUpgradeControllerChartName,
			Namespace: namespace.System,
			Annotations: map[string]string{
				managedSucDeploymentAnno: "true",
			},
		},
	}
	sucDeploymentFromFleetBundle = &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      chart.SystemUpgradeControllerChartName,
			Namespace: namespace.System,
		},
	}
	sucAppNameOverride = "mcc-dev-cluster-system-upgrade-controller"

	localCuster = &v3.Cluster{
		Status: v3.ClusterStatus{
			Driver: "k3s",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "local",
			Annotations: map[string]string{
				importedclusterversionmanagement.VersionManagementAnno: "system-default",
			},
		},
	}
	plans = []*upgradev1.Plan{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "k3s-master-plan",
				Annotations: map[string]string{
					k3sbasedupgrade.RancherManagedPlan: "true",
				},
			},
		},
	}
)

const testYAML = `---
newKey: newValue
mcm:
  enabled: false
global: ""
priorityClassName: newClass
`

type testMocks struct {
	manager                         *chartfake.MockManager
	namespaceCtrl                   *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	namespaceCache                  *fake.MockNonNamespacedCacheInterface[*v1.Namespace]
	configCache                     *fake.MockCacheInterface[*v1.ConfigMap]
	deployment                      *fake.MockControllerInterface[*appsv1.Deployment, *appsv1.DeploymentList]
	deploymentCache                 *fake.MockCacheInterface[*appsv1.Deployment]
	clusterCache                    *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
	plan                            *fake.MockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList]
	planCache                       *fake.MockCacheInterface[*upgradev1.Plan]
	mutatingWebhookConfigurations   *fake.MockNonNamespacedControllerInterface[*admissionv1.MutatingWebhookConfiguration, *admissionv1.MutatingWebhookConfigurationList]
	validatingWebhookConfigurations *fake.MockNonNamespacedControllerInterface[*admissionv1.ValidatingWebhookConfiguration, *admissionv1.ValidatingWebhookConfigurationList]
}

func (t *testMocks) Handler() *handler {
	return &handler{
		manager:                        t.manager,
		namespaces:                     t.namespaceCtrl,
		namespaceCache:                 t.namespaceCache,
		chartsConfig:                   chart.RancherConfigGetter{ConfigCache: t.configCache},
		deployment:                     t.deployment,
		deploymentCache:                t.deploymentCache,
		clusterCache:                   t.clusterCache,
		plan:                           t.plan,
		planCache:                      t.planCache,
		mutatingWebhookConfigurations:  t.mutatingWebhookConfigurations,
		validatingWebhookConfiguration: t.validatingWebhookConfigurations,
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
			name: "normal installation in downstream cluster",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(6)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				features.MCM.Set(false)
				features.MCMAgent.Set(true)
				features.ManagedSystemUpgradeController.Set(true)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				expectedSUCValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.SystemUpgradeControllerChartName,
					chart.SystemUpgradeControllerChartName,
					"",
					"2.0.0",
					expectedSUCValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in downstream cluster with system-upgrade-controller name override",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(6)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				features.MCM.Set(false)
				features.MCMAgent.Set(true)
				features.ManagedSystemUpgradeController.Set(true)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", sucAppNameOverride)

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				expectedSUCValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.SystemUpgradeControllerChartName,
					sucAppNameOverride,
					"",
					"2.0.0",
					expectedSUCValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in downstream cluster with the system-upgrade-controller deployment from old Fleet Bundle",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(4)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeploymentFromFleetBundle, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				features.MCM.Set(false)
				features.MCMAgent.Set(true)
				features.ManagedSystemUpgradeController.Set(true)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				// the Ensure function is not invoked in this case

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in downstream cluster without the system-upgrade-controller deployment",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(4)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(nil, errTest).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				features.MCM.Set(false)
				features.MCMAgent.Set(true)
				features.ManagedSystemUpgradeController.Set(true)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				// the Ensure function is not invoked in this case

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in downstream cluster with imported-cluster-version-management disabled and existing plans",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(4)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				features.MCM.Set(false)
				features.MCMAgent.Set(true)
				features.ManagedSystemUpgradeController.Set(false)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				// the Ensure function is not invoked in this case

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in downstream cluster with imported-cluster-version-management disabled",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(4)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				features.MCM.Set(false)
				features.MCMAgent.Set(true)
				features.ManagedSystemUpgradeController.Set(false)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				mocks.manager.EXPECT().Uninstall(namespace.System, chart.SystemUpgradeControllerChartName).Return(nil)
				mocks.manager.EXPECT().Remove(namespace.System, chart.SystemUpgradeControllerChartName)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in local cluster",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(7)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(2)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.0")
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				expectedSUCValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.SystemUpgradeControllerChartName,
					chart.SystemUpgradeControllerChartName,
					"",
					"2.0.0",
					expectedSUCValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// remotedialer-proxy
				expectedRDPValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.RemoteDialerProxyChartName,
					chart.RemoteDialerProxyChartName,
					"",
					"2.0.0",
					expectedRDPValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in local cluster with imported-cluster-version-managements disabled",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(5)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				_ = settings.ImportedClusterVersionManagement.Set("false")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.0")
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// remotedialer-proxy
				expectedRDPValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.RemoteDialerProxyChartName,
					chart.RemoteDialerProxyChartName,
					"",
					"2.0.0",
					expectedRDPValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				mocks.manager.EXPECT().Uninstall(namespace.System, chart.SystemUpgradeControllerChartName).Return(nil)
				mocks.manager.EXPECT().Remove(namespace.System, chart.SystemUpgradeControllerChartName)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in local cluster with imported-cluster-version-managements disabled and with existing plans",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(5)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(2)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				_ = settings.ImportedClusterVersionManagement.Set("false")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.0")
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// remotedialer-proxy
				expectedRDPValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.RemoteDialerProxyChartName,
					chart.RemoteDialerProxyChartName,
					"",
					"2.0.0",
					expectedRDPValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				// the Ensure function is not invoked in this case

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "normal installation in local cluster with imperative API enabled but RDP disabled",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(4)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(2)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).Times(1)
				os.Setenv("IMPERATIVE_API_DIRECT", "true")
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				_ = settings.ImportedClusterVersionManagement.Set("false")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.0")
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				// the Ensure function is not invoked in this case

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "installation with config cache errors",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(nil, errTest).Times(7)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(2)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.0")
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				expectedSUCValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.SystemUpgradeControllerChartName,
					chart.SystemUpgradeControllerChartName,
					"",
					"2.0.0",
					expectedSUCValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// remotedialer-proxy
				expectedRDPValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.RemoteDialerProxyChartName,
					chart.RemoteDialerProxyChartName,
					"",
					"2.0.0",
					expectedRDPValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
		{
			name: "installation with image override",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(emptyConfig, nil).Times(7)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(2)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.1")
				_ = settings.RancherTurtlesVersion.Set("2.0.1")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.1")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.1")
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.1",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
					"image": map[string]interface{}{
						"repository": "rancher-test.io/rancher/turtles",
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.1",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				// system-upgrade-controller
				expectedSUCValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						}},
					"systemUpgradeController": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "rancher-test.io/rancher/system-upgrade-controller",
						},
					},
					"kubectl": map[string]interface{}{
						"image": map[string]interface{}{
							"repository": "rancher-test.io/rancher/kubectl",
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.SystemUpgradeControllerChartName,
					chart.SystemUpgradeControllerChartName,
					"",
					"2.0.1",
					expectedSUCValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				// remotedialer-proxy
				expectedRDPValues := map[string]interface{}{
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
					"image": map[string]interface{}{
						"repository": "rancher-test.io/rancher/remotedialer-proxy",
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.RemoteDialerProxyChartName,
					chart.RemoteDialerProxyChartName,
					"",
					"2.0.1",
					expectedRDPValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
			registryOverride: "rancher-test.io",
		},
		{
			name: "installation with webhook values",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(fullConfig, nil).Times(7)
				mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(sucDeployment, nil).Times(1)
				mocks.clusterCache.EXPECT().Get("local").Return(localCuster, nil).Times(2)
				mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(nil, nil).Times(1)
				_ = settings.RancherWebhookVersion.Set("2.0.0")
				_ = settings.RancherTurtlesVersion.Set("2.0.0")
				_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
				_ = settings.RemoteDialerProxyVersion.Set("2.0.1")
				features.MCM.Set(true)
				_ = os.Setenv("CATTLE_SUC_APP_NAME_OVERRIDE", "")

				// rancher-webhook
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
					chart.WebhookChartName,
					chart.WebhookChartName,
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-turtles
				expectedTurtlesValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"features": map[string]interface{}{
						"no-cert-manager": map[string]interface{}{
							"enabled": true,
						},
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.TurtlesNamespace,
					chart.TurtlesChartName,
					chart.TurtlesChartName,
					"",
					"2.0.0",
					expectedTurtlesValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// system-upgrade-controller
				expectedSUCValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.SystemUpgradeControllerChartName,
					chart.SystemUpgradeControllerChartName,
					"",
					"2.0.0",
					expectedSUCValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// remotedialer-proxy
				expectedRDPValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					chart.RemoteDialerProxyChartName,
					chart.RemoteDialerProxyChartName,
					"",
					"2.0.1",
					expectedRDPValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				// rancher-operator
				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
				mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset setting to default values before each test
			os.Setenv("IMPERATIVE_API_DIRECT", "")
			_ = settings.RancherWebhookVersion.Set(originalWebhookVersion)
			_ = settings.RancherProvisioningCAPIVersion.Set(originalCAPIVersion)
			_ = settings.SystemUpgradeControllerChartVersion.Set(originalSUCVersion)
			_ = settings.ImportedClusterVersionManagement.Set(originalImportedVersionManagement)
			features.MCM.Set(originalMCM)
			features.MCMAgent.Set(originalMCMAgent)
			features.ManagedSystemUpgradeController.Set(originalManagedSystemUpgradeController)
			features.Turtles.Set(originalTurtles)

			ctrl := gomock.NewController(t)

			// create mocks for each test
			mocks := testMocks{
				manager:         chartfake.NewMockManager(ctrl),
				namespaceCtrl:   fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
				namespaceCache:  fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](ctrl),
				configCache:     fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
				deploymentCache: fake.NewMockCacheInterface[*appsv1.Deployment](ctrl),
				plan:            fake.NewMockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList](ctrl),
				planCache:       fake.NewMockCacheInterface[*upgradev1.Plan](ctrl),
				clusterCache:    fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
			}

			mocks.namespaceCache.EXPECT().Get(namespace.ProvisioningCAPINamespace).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, namespace.ProvisioningCAPINamespace)).AnyTimes()
			mocks.manager.EXPECT().Remove(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName)
			mocks.manager.EXPECT().Uninstall(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName).Return(nil)
			mocks.namespaceCtrl.EXPECT().Delete(namespace.ProvisioningCAPINamespace, nil).Return(nil)

			// allow test to add expected calls to mock and run any additional setup
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

func Test_TurtlesInstallation(t *testing.T) {
	repo := &catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{Name: repoName}}

	os.Setenv("IMPERATIVE_API_DIRECT", "")
	_ = settings.RancherWebhookVersion.Set("2.0.0")
	_ = settings.RancherTurtlesVersion.Set("2.0.0")
	_ = settings.RancherProvisioningCAPIVersion.Set("2.0.0")
	_ = settings.SystemUpgradeControllerChartVersion.Set("2.0.0")
	features.Turtles.Set(true)
	features.MCMAgent.Set(true)
	features.MCM.Set(false)

	ctrl := gomock.NewController(t)
	mocks := testMocks{
		manager:         chartfake.NewMockManager(ctrl),
		namespaceCtrl:   fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
		namespaceCache:  fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](ctrl),
		configCache:     fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
		deploymentCache: fake.NewMockCacheInterface[*appsv1.Deployment](ctrl),
		plan:            fake.NewMockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList](ctrl),
		planCache:       fake.NewMockCacheInterface[*upgradev1.Plan](ctrl),
		clusterCache:    fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
	}

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).AnyTimes()
	mocks.namespaceCache.EXPECT().Get(namespace.ProvisioningCAPINamespace).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, namespace.ProvisioningCAPINamespace)).AnyTimes()
	features.ManagedSystemUpgradeController.Set(false)
	mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).Times(1)
	mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, sucDeploymentName)).AnyTimes()

	expectedWebhookValues := map[string]interface{}{
		"priorityClassName": priorityClassName,
		"capi":              nil,
		"mcm":               map[string]interface{}{"enabled": features.MCM.Enabled()},
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{"systemDefaultRegistry": settings.SystemDefaultRegistry.Get()},
		},
	}
	expectedTurtlesValues := map[string]interface{}{
		"priorityClassName": priorityClassName,
		"features": map[string]interface{}{
			"no-cert-manager": map[string]interface{}{
				"enabled": true,
			},
		},
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{"systemDefaultRegistry": settings.SystemDefaultRegistry.Get()},
		},
	}
	gomock.InOrder(
		mocks.manager.EXPECT().Ensure(
			namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", "2.0.0", expectedWebhookValues, gomock.AssignableToTypeOf(false), "",
		).Return(nil),

		mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator"),
		mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil),

		mocks.manager.EXPECT().Remove(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName),
		mocks.manager.EXPECT().Uninstall(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName).Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(namespace.ProvisioningCAPINamespace, nil).Return(nil),

		mocks.manager.EXPECT().Ensure(
			namespace.TurtlesNamespace, chart.TurtlesChartName, chart.TurtlesChartName, "", "2.0.0", expectedTurtlesValues, gomock.AssignableToTypeOf(false), "",
		).Return(nil),
	)

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func Test_SwitchFromProvisioningToTurtlesRace(t *testing.T) {
	repo := &catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{Name: repoName}}
	os.Setenv("IMPERATIVE_API_DIRECT", "")
	_ = settings.RancherWebhookVersion.Set("2.0.0")
	_ = settings.RancherTurtlesVersion.Set("2.0.0")
	_ = settings.RancherProvisioningCAPIVersion.Set("2.0.0")
	features.Turtles.Set(true)
	features.MCMAgent.Set(true)
	features.MCM.Set(false)

	ctrl := gomock.NewController(t)
	mocks := testMocks{
		manager:         chartfake.NewMockManager(ctrl),
		namespaceCtrl:   fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
		namespaceCache:  fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](ctrl),
		configCache:     fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
		deploymentCache: fake.NewMockCacheInterface[*appsv1.Deployment](ctrl),
		plan:            fake.NewMockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList](ctrl),
		planCache:       fake.NewMockCacheInterface[*upgradev1.Plan](ctrl),
		clusterCache:    fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
	}

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).AnyTimes()

	features.ManagedSystemUpgradeController.Set(false)
	mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).Times(2)
	mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, sucDeploymentName)).AnyTimes()

	gomock.InOrder(
		mocks.manager.EXPECT().Ensure(
			namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", "2.0.0",
			gomock.AssignableToTypeOf(map[string]interface{}{}), gomock.AssignableToTypeOf(false), "",
		).Return(nil),

		mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator"),
		mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil),

		mocks.manager.EXPECT().Remove(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName),
		mocks.manager.EXPECT().Uninstall(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName).Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(namespace.ProvisioningCAPINamespace, nil).Return(nil),
		mocks.namespaceCache.EXPECT().Get(namespace.ProvisioningCAPINamespace).Return(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace.ProvisioningCAPINamespace}}, nil),
	)
	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)

	expectedTurtlesValues := map[string]interface{}{
		"priorityClassName": priorityClassName,
		"features": map[string]interface{}{
			"no-cert-manager": map[string]interface{}{
				"enabled": true,
			},
		},
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{"systemDefaultRegistry": settings.SystemDefaultRegistry.Get()},
		},
	}
	gomock.InOrder(
		mocks.manager.EXPECT().Ensure(
			namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", "2.0.0",
			gomock.AssignableToTypeOf(map[string]interface{}{}), gomock.AssignableToTypeOf(false), "",
		).Return(nil),

		mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator"),
		mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil),

		mocks.manager.EXPECT().Remove(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName),
		mocks.manager.EXPECT().Uninstall(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName).Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(namespace.ProvisioningCAPINamespace, nil).Return(nil),

		mocks.namespaceCache.EXPECT().Get(namespace.ProvisioningCAPINamespace).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, namespace.ProvisioningCAPINamespace)),
		mocks.manager.EXPECT().Ensure(
			namespace.TurtlesNamespace, chart.TurtlesChartName, chart.TurtlesChartName, "", "2.0.0", expectedTurtlesValues, gomock.AssignableToTypeOf(false), "",
		).Return(nil),
	)

	_, err = h.onRepo("", repo)
	require.NoError(t, err)
}

func Test_SwitchFromTurtlesToProvisioningRace(t *testing.T) {
	repo := &catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{Name: repoName}}
	os.Setenv("IMPERATIVE_API_DIRECT", "")
	_ = settings.RancherWebhookVersion.Set("2.0.0")
	_ = settings.RancherTurtlesVersion.Set("2.0.0")
	_ = settings.RancherProvisioningCAPIVersion.Set("2.0.0")

	features.EmbeddedClusterAPI.Set(true)
	features.Turtles.Set(false)
	features.MCMAgent.Set(true)
	features.MCM.Set(false)
	features.ManagedSystemUpgradeController.Set(false)

	ctrl := gomock.NewController(t)
	mocks := testMocks{
		manager:         chartfake.NewMockManager(ctrl),
		namespaceCtrl:   fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
		namespaceCache:  fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](ctrl),
		configCache:     fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
		deploymentCache: fake.NewMockCacheInterface[*appsv1.Deployment](ctrl),
		plan:            fake.NewMockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList](ctrl),
		planCache:       fake.NewMockCacheInterface[*upgradev1.Plan](ctrl),
		clusterCache:    fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
	}

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).AnyTimes()

	mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).AnyTimes()
	mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, sucDeploymentName)).AnyTimes()

	mocks.namespaceCache.EXPECT().Get(namespace.TurtlesNamespace).Return(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace.TurtlesNamespace}}, nil)
	gomock.InOrder(
		mocks.manager.EXPECT().Ensure(
			namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", "2.0.0",
			gomock.AssignableToTypeOf(map[string]interface{}{}), gomock.AssignableToTypeOf(false), "",
		).Return(nil),

		mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator"),
		mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil),

		mocks.manager.EXPECT().Remove(namespace.TurtlesNamespace, chart.TurtlesChartName),
		mocks.manager.EXPECT().Uninstall(namespace.TurtlesNamespace, chart.TurtlesChartName).Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(namespace.TurtlesNamespace, nil).Return(nil),
	)
	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)

	expectedProvCAPIValues := map[string]interface{}{
		"priorityClassName": priorityClassName,
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{"systemDefaultRegistry": settings.SystemDefaultRegistry.Get()},
		},
	}
	gomock.InOrder(
		mocks.manager.EXPECT().Ensure(
			namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", "2.0.0",
			gomock.AssignableToTypeOf(map[string]interface{}{}), gomock.AssignableToTypeOf(false), "",
		).Return(nil),

		mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator"),
		mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil),

		mocks.namespaceCache.EXPECT().Get(namespace.TurtlesNamespace).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, namespace.TurtlesNamespace)),
		mocks.manager.EXPECT().Ensure(
			namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName, chart.ProvisioningCAPIChartName, "", "2.0.0",
			expectedProvCAPIValues, gomock.AssignableToTypeOf(false), "",
		).Return(nil),
		mocks.manager.EXPECT().Remove(namespace.TurtlesNamespace, chart.TurtlesChartName),
		mocks.manager.EXPECT().Uninstall(namespace.TurtlesNamespace, chart.TurtlesChartName).Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(namespace.TurtlesNamespace, nil).Return(nil),
	)
	_, err = h.onRepo("", repo)
	require.NoError(t, err)
}

func Test_TurtlesWinsWhenBothEnabled(t *testing.T) {
	repo := &catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{Name: repoName}}
	_ = settings.RancherTurtlesVersion.Set("2.0.0")
	_ = settings.RancherProvisioningCAPIVersion.Set("2.0.0")
	features.Turtles.Set(true)
	features.EmbeddedClusterAPI.Set(true)
	features.MCMAgent.Set(true)
	features.MCM.Set(false)
	features.ManagedSystemUpgradeController.Set(false)

	ctrl := gomock.NewController(t)
	mocks := testMocks{
		manager:         chartfake.NewMockManager(ctrl),
		namespaceCtrl:   fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
		namespaceCache:  fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](ctrl),
		configCache:     fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
		deploymentCache: fake.NewMockCacheInterface[*appsv1.Deployment](ctrl),
		plan:            fake.NewMockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList](ctrl),
		planCache:       fake.NewMockCacheInterface[*upgradev1.Plan](ctrl),
		clusterCache:    fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
	}

	mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).AnyTimes()

	mocks.planCache.EXPECT().List(namespace.System, managedPlanSelector).Return(plans, nil).AnyTimes()
	mocks.deploymentCache.EXPECT().Get(namespace.System, sucDeploymentName).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, sucDeploymentName)).AnyTimes()

	mocks.namespaceCache.EXPECT().Get(namespace.ProvisioningCAPINamespace).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "namespaces"}, namespace.ProvisioningCAPINamespace)).AnyTimes()
	expectedTurtlesValues := map[string]interface{}{
		"priorityClassName": priorityClassName,
		"features": map[string]interface{}{
			"no-cert-manager": map[string]interface{}{
				"enabled": true,
			},
		},
		"global": map[string]interface{}{
			"cattle": map[string]interface{}{"systemDefaultRegistry": settings.SystemDefaultRegistry.Get()},
		},
	}
	gomock.InOrder(
		mocks.manager.EXPECT().Ensure(
			namespace.System, chart.WebhookChartName, chart.WebhookChartName, "", "2.0.0",
			gomock.AssignableToTypeOf(map[string]interface{}{}), gomock.AssignableToTypeOf(false), "",
		).Return(nil),

		mocks.manager.EXPECT().Remove(operatorNamespace, "rancher-operator"),
		mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil),

		mocks.manager.EXPECT().Remove(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName),
		mocks.manager.EXPECT().Uninstall(namespace.ProvisioningCAPINamespace, chart.ProvisioningCAPIChartName).Return(nil),
		mocks.namespaceCtrl.EXPECT().Delete(namespace.ProvisioningCAPINamespace, nil).Return(nil),

		mocks.manager.EXPECT().Ensure(
			namespace.TurtlesNamespace, chart.TurtlesChartName, chart.TurtlesChartName, "", "2.0.0",
			expectedTurtlesValues, gomock.AssignableToTypeOf(false), "",
		).Return(nil),
	)

	h := mocks.Handler()
	_, err := h.onRepo("", repo)
	require.NoError(t, err)
}

func Test_CleanupEmbeddedWebhookConfigs(t *testing.T) {
	expectNoConfigsDeleted := func() *testMocks {
		ctrl := gomock.NewController(t)
		mocks := testMocks{
			validatingWebhookConfigurations: fake.NewMockNonNamespacedControllerInterface[*admissionv1.ValidatingWebhookConfiguration, *admissionv1.ValidatingWebhookConfigurationList](ctrl),
			mutatingWebhookConfigurations:   fake.NewMockNonNamespacedControllerInterface[*admissionv1.MutatingWebhookConfiguration, *admissionv1.MutatingWebhookConfigurationList](ctrl),
		}
		gomock.InOrder(
			mocks.mutatingWebhookConfigurations.EXPECT().Delete(capiMutatingWebhookName, nil).Times(0),
			mocks.validatingWebhookConfigurations.EXPECT().Delete(capiValidatingWebhookName, nil).Times(0),
		)
		return &mocks
	}

	tests := []struct {
		name              string
		namespaceName     string
		deletionTime      *metav1.Time
		setupExpectations func() *testMocks
	}{
		{
			name:              "unrelated NS",
			namespaceName:     "not-embedded-capi",
			setupExpectations: expectNoConfigsDeleted,
		},
		{
			name:              "unrelated NS being deleted",
			namespaceName:     "not-embedded-capi",
			deletionTime:      &metav1.Time{Time: time.Now()},
			setupExpectations: expectNoConfigsDeleted,
		},
		{
			name:              "embedded capi namespace non-deletion",
			namespaceName:     namespace.ProvisioningCAPINamespace,
			setupExpectations: expectNoConfigsDeleted,
		},
		{
			name:          "embedded-capi namespace deletion",
			namespaceName: namespace.ProvisioningCAPINamespace,
			deletionTime:  &metav1.Time{Time: time.Now()},
			setupExpectations: func() *testMocks {
				ctrl := gomock.NewController(t)
				mocks := testMocks{
					validatingWebhookConfigurations: fake.NewMockNonNamespacedControllerInterface[*admissionv1.ValidatingWebhookConfiguration, *admissionv1.ValidatingWebhookConfigurationList](ctrl),
					mutatingWebhookConfigurations:   fake.NewMockNonNamespacedControllerInterface[*admissionv1.MutatingWebhookConfiguration, *admissionv1.MutatingWebhookConfigurationList](ctrl),
				}
				gomock.InOrder(
					mocks.mutatingWebhookConfigurations.EXPECT().Delete(capiMutatingWebhookName, &metav1.DeleteOptions{}).Times(1),
					mocks.validatingWebhookConfigurations.EXPECT().Delete(capiValidatingWebhookName, &metav1.DeleteOptions{}).Times(1),
				)
				return &mocks
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			testNS := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tst.namespaceName}}
			if tst.deletionTime != nil {
				testNS.DeletionTimestamp = tst.deletionTime
			}
			mocks := tst.setupExpectations()
			h := mocks.Handler()
			_, err := h.cleanUpEmbeddedCAPIWebhooks("", testNS)
			require.NoError(t, err)
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
			name: "turtles chart version",
			changedObj: &v3.Setting{ObjectMeta: metav1.ObjectMeta{
				Name: settings.RancherTurtlesVersion.Name,
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
