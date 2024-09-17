package managesystemagent

import (
	"testing"
	"time"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	fleetv1alpha1 "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManageSystemAgent_syncSystemUpgradeControllerStatusConditionManipulation(t *testing.T) {
	type args struct {
		controlPlaneName      string
		controlPlaneNamespace string
		kubernetesVersion     string
		changeExpected        bool
		bs                    fleetv1alpha1.BundleSummary
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "basic bundle ready test case",
			args: args{
				controlPlaneName:      "lol",
				controlPlaneNamespace: "lol",
				kubernetesVersion:     "v1.25.5+k3s1",
				bs: fleetv1alpha1.BundleSummary{
					DesiredReady: 1,
					Ready:        1,
				},
			},
		},
		{
			name: "test for super long controlplane name",
			args: args{
				controlPlaneName:      "ayyhxrojzehfiqampacgkqbqyewdjxwvhjowpikqqtxbkjqpegqaovgfehehkfg",
				controlPlaneNamespace: "lol",
				kubernetesVersion:     "v1.26.5+k3s1",
				bs: fleetv1alpha1.BundleSummary{
					DesiredReady: 1,
					Ready:        1,
				},
				changeExpected: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bc := fake.NewMockControllerInterface[*v1alpha1.Bundle, *v1alpha1.BundleList](ctrl)
			pc := fake.NewMockCacheInterface[*v1.Cluster](ctrl)
			h := &handler{
				bundles:      bc,
				provClusters: pc,
			}
			a := assert.New(t)

			mockControlPlane := &rkev1.RKEControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.args.controlPlaneName,
					Namespace: tt.args.controlPlaneNamespace,
				},
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: tt.args.kubernetesVersion,
				},
			}

			capr.SystemUpgradeControllerReady.True(&mockControlPlane.Status)
			capr.SystemUpgradeControllerReady.Message(&mockControlPlane.Status, "")
			// Set the "last updated time" to the start of time, because RFC3339 only provides granularity at seconds and the test can run in less than a second (thus ensuring the timestamp is mutated when we expect it to be mutated)
			capr.SystemUpgradeControllerReady.LastUpdated(&mockControlPlane.Status, time.Time{}.UTC().Format(time.RFC3339))
			lu := capr.SystemUpgradeControllerReady.GetLastUpdated(&mockControlPlane.Status)

			pc.EXPECT().Get(tt.args.controlPlaneNamespace, tt.args.controlPlaneName).Return(&v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.args.controlPlaneName,
					Namespace: tt.args.controlPlaneNamespace,
				},
				Status: v1.ClusterStatus{
					FleetWorkspaceName: tt.args.controlPlaneNamespace,
				},
			}, nil)
			expectedBundleName := capr.SafeConcatName(capr.MaxHelmReleaseNameLength, "mcc", capr.SafeConcatName(48, tt.args.controlPlaneName, "managed", "system-upgrade-controller"))
			bc.EXPECT().Get(tt.args.controlPlaneNamespace, expectedBundleName, metav1.GetOptions{}).Return(&fleetv1alpha1.Bundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedBundleName,
					Namespace: tt.args.controlPlaneNamespace,
				},
				Spec: fleetv1alpha1.BundleSpec{
					BundleDeploymentOptions: fleetv1alpha1.BundleDeploymentOptions{
						Helm: &fleetv1alpha1.HelmOptions{
							Values: &fleetv1alpha1.GenericMap{
								Data: map[string]interface{}{
									"global": map[string]interface{}{
										"cattle": map[string]interface{}{},
									},
								},
							},
						},
					},
				},
				Status: fleetv1alpha1.BundleStatus{
					Summary: tt.args.bs,
				},
			}, nil)

			resultingStatus, err := h.syncSystemUpgradeControllerStatus(mockControlPlane, mockControlPlane.Status)
			a.NoError(err)
			if tt.args.changeExpected {
				a.NotEqual(lu, capr.SystemUpgradeControllerReady.GetLastUpdated(&resultingStatus))
			} else {
				a.Equal(lu, capr.SystemUpgradeControllerReady.GetLastUpdated(&resultingStatus))
			}
		})
	}
}
