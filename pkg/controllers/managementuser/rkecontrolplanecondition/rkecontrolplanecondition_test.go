package rkecontrolplanecondition

import (
	"fmt"
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
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
)

type setupConfig struct {
	// The mgmt cluster that the mock handler is registered for
	mgmtClusterName string

	// The objects that the mock handler returns
	app *catalog.App

	// The error that the mock handler returns
	appError error

	// The value for the SystemUpgradeControllerChartVersion setting
	chartVersion string
}

type testCase struct {
	name  string
	setup setupConfig
	input *v1.RKEControlPlane

	// Expected results
	wantError             bool
	wantedConditionStatus string
	appClientIsInvoked    bool
	enqueueAfterIsInvoked bool
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
		},
		{
			name: "chart version is not set",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
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
		},
		{
			name: "agent is not connected",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: false,
				},
			},
			wantError:             false,
			wantedConditionStatus: "",
			appClientIsInvoked:    false,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             true,
			wantedConditionStatus: "",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "True",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    true,
		},
		{
			name: "condition recently updated - downstream API call is skipped",
			setup: setupConfig{
				mgmtClusterName: mgmtClusterName,
				chartVersion:    "160.1.0",
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
					Conditions: []genericcondition.GenericCondition{
						{
							Type:           "SystemUpgradeControllerReady",
							Status:         "False",
							LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
						},
					},
				},
			},
			wantError:             false,
			wantedConditionStatus: "False",
			appClientIsInvoked:    false,
			enqueueAfterIsInvoked: true,
		},
		{
			name: "condition updated more than 30 seconds ago - downstream API call proceeds",
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
				Status: v1.RKEControlPlaneStatus{
					AgentConnected: true,
					Conditions: []genericcondition.GenericCondition{
						{
							Type:           "SystemUpgradeControllerReady",
							Status:         "False",
							LastUpdateTime: time.Now().Add(-35 * time.Second).UTC().Format(time.RFC3339),
						},
					},
				},
			},
			wantError:             false,
			wantedConditionStatus: "True",
			appClientIsInvoked:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bc := fake.NewMockControllerInterface[*catalog.App, *catalog.AppList](ctrl)
			rc := fake.NewMockControllerInterface[*v1.RKEControlPlane, *v1.RKEControlPlaneList](ctrl)
			h := &handler{
				mgmtClusterName:           tt.setup.mgmtClusterName,
				downstreamAppClient:       bc,
				rkeControlPlaneController: rc,
			}
			if tt.appClientIsInvoked {
				bc.EXPECT().Get(namespace.System, appName(tt.input.Spec.ClusterName), metav1.GetOptions{}).Return(tt.setup.app, tt.setup.appError)
			}
			if tt.enqueueAfterIsInvoked {
				rc.EXPECT().EnqueueAfter(tt.input.Namespace, tt.input.Name, gomock.Any()).Times(1)
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
			got, err := h.syncSystemUpgradeControllerCondition(tt.input, tt.input.Status)

			if (err != nil) != tt.wantError {
				t.Errorf("syncSystemUpgradeControllerCondition() error = %v, wantError %v", err, tt.wantError)
				return
			}
			// Check the condition's status value instead of the entire object,
			// as it includes a lastUpdateTime field that is difficult to mock
			if capr.SystemUpgradeControllerReady.GetStatus(&got) != tt.wantedConditionStatus {
				t.Errorf("syncSystemUpgradeControllerCondition() got = %v, expected SystemUpgradeControllerReady condition status value = %v", got, tt.wantedConditionStatus)
			}
		})
	}
}
