package rkecontrolplanecondition

import (
	"fmt"
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	prov "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/managesystemagent"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"go.uber.org/mock/gomock"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	mgmtClusterName  = "c-mx21351"
	provClusterName  = "dev-cluster"
	controlPlaneName = "cp-58542"

	basicCluster = &prov.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      provClusterName,
			Namespace: namespace.System,
		},
		Status: prov.ClusterStatus{
			Conditions: []genericcondition.GenericCondition{
				{
					Type:   "Connected",
					Status: "True",
				},
			},
		},
	}
)

type setupConfig struct {
	// The mgmt cluster that the mock handler is registered for
	mgmtClusterName string

	// The objects that the mock handler returns
	app     *catalog.App
	plan    *upgradev1.Plan
	cluster *prov.Cluster

	// The error that the mock handler returns
	appError     error
	clusterError error
	planError    error

	// The value for the SystemUpgradeControllerChartVersion setting
	chartVersion string
	// The value for the SystemAgentUpgradeImage setting
	image string
}

type testCase struct {
	name  string
	setup setupConfig
	input *v1.RKEControlPlane

	// Expected results
	wantError             bool
	wantedConditionStatus string
	appClientIsInvoked    bool
	planClientIsInvoked   bool
	clusterCacheIsInvoked bool
}

func Test_handler_syncSystemUpgradeControllerStatus(t *testing.T) {
	tests := []testCase{
		{
			name: "rkeControlPlane is being deleted",
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:              controlPlaneName,
					Namespace:         namespace.System,
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName: provClusterName,
				},
			},
			wantError:             false,
			wantedConditionStatus: "",
			appClientIsInvoked:    false,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: false,
		},
		{
			name: "rkeControlPlane is for a different cluster",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ManagementClusterName: "another-cluster",
				},
			},
			wantError:             false,
			wantedConditionStatus: "",
			appClientIsInvoked:    false,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: false,
		},
		{
			name: "chart version is not set",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				cluster:         basicCluster,
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ManagementClusterName: mgmtClusterName,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    false,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: false,
		},
		{
			name: "fail to get cluster",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				cluster:         basicCluster,
				clusterError:    fmt.Errorf("some error"),
				chartVersion:    "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ManagementClusterName: mgmtClusterName,
				},
			},
			wantError:             true,
			wantedConditionStatus: "",
			appClientIsInvoked:    false,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "cluster is not connected",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				cluster: &prov.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      provClusterName,
						Namespace: namespace.System,
					},
					Status: prov.ClusterStatus{
						Conditions: []genericcondition.GenericCondition{
							{
								Type:   "Connected",
								Status: "False",
							},
						},
					},
				},
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ManagementClusterName: mgmtClusterName,
				},
			},
			wantError:             false,
			wantedConditionStatus: "",
			appClientIsInvoked:    false,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "fail to get the app with notFound error",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-app",
						Namespace: namespace.System,
					},
					Spec:   catalog.ReleaseSpec{},
					Status: catalog.ReleaseStatus{},
				},
				cluster:      basicCluster,
				appError:     apierror.NewNotFound(catalog.Resource("app"), appName(provClusterName)),
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ManagementClusterName: mgmtClusterName,
					ClusterName:           provClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "fail to get the app with non-notFound error",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appName(provClusterName),
						Namespace: namespace.System,
					},
					Spec:   catalog.ReleaseSpec{},
					Status: catalog.ReleaseStatus{},
				},
				cluster:      basicCluster,
				appError:     apierror.NewInternalError(fmt.Errorf("something goes wrong")),
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ManagementClusterName: mgmtClusterName,
					ClusterName:           provClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             true,
			wantedConditionStatus: "",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "app is being deleted",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:              appName(provClusterName),
						Namespace:         namespace.System,
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
					Spec:   catalog.ReleaseSpec{},
					Status: catalog.ReleaseStatus{},
				},
				cluster:      basicCluster,
				appError:     nil,
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName:           provClusterName,
					ManagementClusterName: mgmtClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "app's chart version is out of sync",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appName(provClusterName),
						Namespace: namespace.System,
					},
					Spec: catalog.ReleaseSpec{
						Chart: &catalog.Chart{
							Metadata: &catalog.Metadata{
								Version: "160.0.0",
							},
						},
					},
					Status: catalog.ReleaseStatus{},
				},
				cluster:      basicCluster,
				appError:     nil,
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName:           provClusterName,
					ManagementClusterName: mgmtClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "app is deployed",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appName(provClusterName),
						Namespace: namespace.System,
					},
					Spec: catalog.ReleaseSpec{
						Chart: &catalog.Chart{
							Metadata: &catalog.Metadata{
								Version: "160.1.0",
							},
						},
					},
					Status: catalog.ReleaseStatus{
						Summary: catalog.Summary{
							State:         string(catalog.StatusDeployed),
							Error:         false,
							Transitioning: false,
						},
					},
				},
				cluster:      basicCluster,
				appError:     nil,
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName:           provClusterName,
					ManagementClusterName: mgmtClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "True",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "app is in failed state",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appName(provClusterName),
						Namespace: namespace.System,
					},
					Spec: catalog.ReleaseSpec{
						Chart: &catalog.Chart{
							Metadata: &catalog.Metadata{
								Version: "160.1.0",
							},
						},
					},
					Status: catalog.ReleaseStatus{
						Summary: catalog.Summary{
							State:         string(catalog.StatusFailed),
							Error:         true,
							Transitioning: false,
						},
					},
				},
				cluster:      basicCluster,
				appError:     nil,
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName:           provClusterName,
					ManagementClusterName: mgmtClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "app is transitioning",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appName(provClusterName),
						Namespace: namespace.System,
					},
					Spec: catalog.ReleaseSpec{
						Chart: &catalog.Chart{
							Metadata: &catalog.Metadata{
								Version: "160.1.0",
							},
						},
					},
					Status: catalog.ReleaseStatus{
						Summary: catalog.Summary{
							State:         string(catalog.StatusPendingInstall),
							Error:         false,
							Transitioning: true,
						},
					},
				},
				cluster:      basicCluster,
				appError:     nil,
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName:           provClusterName,
					ManagementClusterName: mgmtClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
		{
			name: "app is uninstalled",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				app: &catalog.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      appName(provClusterName),
						Namespace: namespace.System,
					},
					Spec: catalog.ReleaseSpec{
						Chart: &catalog.Chart{
							Metadata: &catalog.Metadata{
								Version: "160.1.0",
							},
						},
					},
					Status: catalog.ReleaseStatus{
						Summary: catalog.Summary{
							State:         string(catalog.StatusUninstalled),
							Error:         false,
							Transitioning: false,
						},
					},
				},
				cluster:      basicCluster,
				appError:     nil,
				chartVersion: "160.1.0",
			},
			input: &v1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controlPlaneName,
					Namespace: namespace.System,
				},
				Spec: v1.RKEControlPlaneSpec{
					ClusterName:           provClusterName,
					ManagementClusterName: mgmtClusterName,
				},
				Status: v1.RKEControlPlaneStatus{},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
			planClientIsInvoked:   false,
			clusterCacheIsInvoked: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bc := fake.NewMockControllerInterface[*catalog.App, *catalog.AppList](ctrl)
			cc := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
			pc := fake.NewMockControllerInterface[*upgradev1.Plan, *upgradev1.PlanList](ctrl)
			h := &handler{
				MgmtClusterName:      tt.setup.mgmtClusterName,
				DownstreamAppClient:  bc,
				DownstreamPlanClient: pc,
				ClusterCache:         cc,
			}
			if tt.appClientIsInvoked {
				bc.EXPECT().Get(namespace.System, appName(tt.input.Spec.ClusterName), metav1.GetOptions{}).Return(tt.setup.app, tt.setup.appError)
			}
			if tt.clusterCacheIsInvoked {
				cc.EXPECT().GetByIndex(cluster.ByCluster, tt.setup.mgmtClusterName).Return([]*prov.Cluster{tt.setup.cluster}, tt.setup.clusterError)
			}
			if tt.planClientIsInvoked {
				pc.EXPECT().Get(namespace.System, managesystemagent.SystemAgentUpgrader, metav1.GetOptions{}).Return(tt.setup.plan, tt.setup.appError)
			}

			if tt.setup.chartVersion != "" {
				current := settings.SystemUpgradeControllerChartVersion.Get()
				if err := settings.SystemUpgradeControllerChartVersion.Set(tt.setup.chartVersion); err != nil {
					t.Errorf("failed to set up : %v", err)
				}
				defer func() {
					err := settings.SystemUpgradeControllerChartVersion.Set(current)
					if err != nil {

					}
				}()
			}
			got, err := h.SyncSystemUpgradeControllerCondition(tt.input, tt.input.Status)

			if (err != nil) != tt.wantError {
				t.Errorf("SyncSystemUpgradeControllerCondition() error = %v, wantError %v", err, tt.wantError)
				return
			}
			// Check the condition's status value instead of the entire object,
			// as it includes a lastUpdateTime field that is difficult to mock
			if capr.SystemUpgradeControllerReady.GetStatus(&got) != tt.wantedConditionStatus {
				t.Errorf("SyncSystemUpgradeControllerCondition() got = %v, expected SystemUpgradeControllerReady condition status value = %v", got, tt.wantedConditionStatus)
			}
		})
	}
}
