package system

// mockgen --build_flags=--mod=mod -package system -destination=./mock_system_test.go github.com/rancher/rancher/pkg/catalogv2/system ContentClient,OperationClient,HelmClient

import (
	context "context"
	"errors"
	"testing"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManagerRemovesRelease(t *testing.T) {
	t.Parallel()
	webhook := desiredKey{
		namespace:            "cattle-system",
		name:                 "rancher-webhook",
		minVersion:           "1.1.1",
		exactVersion:         "1.2.0",
		installImageOverride: "some-image",
	}
	charts := map[desiredKey]map[string]any{
		webhook: {"foo": "bar"},
	}

	manager := Manager{desiredCharts: charts}
	manager.Remove("hello", "world")
	assert.Equal(t, charts, manager.desiredCharts)

	manager.Remove("cattle-system", "rancher-webhook")
	assert.Equal(t, map[desiredKey]map[string]any{}, manager.desiredCharts)

	// Assert that the lookup of key to delete only needs namespace and name.
	webhook = desiredKey{
		namespace: "cattle-system",
		name:      "rancher-webhook",
	}
	charts[webhook] = map[string]any{}
	assert.Equal(t, charts, manager.desiredCharts)
	manager.Remove("cattle-system", "rancher-webhook")
	assert.Equal(t, map[desiredKey]map[string]any{}, manager.desiredCharts)
}

func TestStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ctx := context.Background()

	mockSettingsController := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](ctrl)
	mockSettingsController.EXPECT().OnChange(ctx, "system-feature-chart-refresh", gomock.Any())

	mockClusterRepos := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
	mockClusterRepos.EXPECT().OnChange(ctx, "catalog-refresh-trigger", gomock.Any())

	manager := Manager{
		settings:     mockSettingsController,
		clusterRepos: mockClusterRepos,
	}

	manager.Start(ctx)
}

func TestInstallCharts(t *testing.T) {
	var (
		fleetChartV1 = release.Release{
			Namespace: "cattle-fleet-system",
			Name:      "fleet",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "1.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		fleetChartV2 = release.Release{
			Namespace: "cattle-fleet-system",
			Name:      "fleet",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "2.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		fleetChartV3 = release.Release{
			Namespace: "cattle-fleet-system",
			Name:      "fleet",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "3.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		rancherChartV1 = release.Release{
			Namespace: "cattle-system",
			Name:      "rancher-webhook",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "1.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		aksOperatorChartV1 = release.Release{
			Namespace: "cattle-fleet-system",
			Name:      "aks-operator",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "1.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		aksOperatorChartV2 = release.Release{
			Namespace: "cattle-fleet-system",
			Name:      "aks-operator",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "2.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		aksOperatorChartV3 = release.Release{
			Namespace: "cattle-fleet-system",
			Name:      "aks-operator",
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "3.0.0",
				},
			},
			Info: &release.Info{
				Status: release.StatusDeployed,
			},
		}
		fleetRepoV1 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "1.0.0",
			},
			URLs: []string{"foo"},
		}
		fleetRepoV2 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "2.0.0",
			},
			URLs: []string{"foo"},
		}
		fleetRepoV3 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "3.0.0",
			},
			URLs: []string{"foo"},
		}
		aksOperatorRepoV1 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "1.0.0",
			},
			URLs: []string{"foo"},
		}
		aksOperatorRepoV2 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "2.0.0",
			},
			URLs: []string{"foo"},
		}
		aksOperatorRepoV3 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "3.0.0",
			},
			URLs: []string{"foo"},
		}
		rancherRepoV1 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "1.0.0",
			},
			URLs: []string{"foo"},
		}
		rancherRepoV2 = repo.ChartVersion{
			Metadata: &chart.Metadata{
				Version: "2.0.0",
			},
			URLs: []string{"foo"},
		}
	)

	tests := []struct {
		name            string
		releases        []*release.Release
		indexedReleases map[string]repo.ChartVersions
		desiredCharts   map[desiredKey]map[string]any
		takeOwnership   bool
		expectInstalls  map[string]bool
		expectedErr     error
	}{
		{
			name:     "Updates charts to desired version",
			releases: []*release.Release{&rancherChartV1, &fleetChartV1, &aksOperatorChartV1},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV2},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV2},
			},
			desiredCharts: map[desiredKey]map[string]any{
				{
					namespace:    "cattle-fleet-system",
					name:         "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			takeOwnership: false,
			expectInstalls: map[string]bool{
				"rancher-webhook": true,
				"fleet":           true,
				"aks-operator":    true,
			},
		},
		{
			name:     "Keeps installed release matching desired version",
			releases: []*release.Release{&fleetChartV2, &rancherChartV1, &aksOperatorChartV2},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV2},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV2},
			},
			desiredCharts: map[desiredKey]map[string]any{
				{
					namespace:    "cattle-fleet-system",
					name:         "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			takeOwnership: false,
			expectInstalls: map[string]bool{
				"fleet":           false,
				"rancher-webhook": true,
				"aks-operator":    true,
			},
		},
		{
			name:     "Keeps installed release, more recent than desired version",
			releases: []*release.Release{&rancherChartV1, &fleetChartV3, &aksOperatorChartV3},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV2, &fleetRepoV3},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV2, &aksOperatorRepoV3},
			},
			desiredCharts: map[desiredKey]map[string]any{
				{
					namespace:    "cattle-fleet-system",
					name:         "fleet",
					minVersion:   "1.0.0",
					exactVersion: "",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			takeOwnership: false,
			expectInstalls: map[string]bool{
				"fleet":           false,
				"rancher-webhook": true,
				"aks-operator":    true,
			},
		},
		{
			// This use case can occur when restoring to an older Rancher version.
			name:     "Downgrades installed release, more recent than desired version",
			releases: []*release.Release{&rancherChartV1, &fleetChartV3, &aksOperatorChartV3},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV2, &fleetRepoV3},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV2, &aksOperatorRepoV3},
			},
			desiredCharts: map[desiredKey]map[string]any{
				{
					namespace:    "cattle-fleet-system",
					name:         "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			takeOwnership: false,
			expectInstalls: map[string]bool{
				"fleet":           true,
				"rancher-webhook": true,
				"aks-operator":    true,
			},
		},
		{
			name:     "Installs release if none installed",
			releases: []*release.Release{&rancherChartV1},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV2},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV2},
			},
			desiredCharts: map[desiredKey]map[string]any{
				{
					namespace:    "cattle-fleet-system",
					name:         "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			takeOwnership: false,
			expectInstalls: map[string]bool{
				"fleet":           true,
				"rancher-webhook": true,
				"aks-operator":    true,
			},
		},
		{
			name:     "Fails to install specified release if not found in catalog",
			releases: []*release.Release{&rancherChartV1},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV3},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV3},
			},
			desiredCharts: map[desiredKey]map[string]any{
				{
					namespace: "cattle-fleet-system",
					name:      "fleet",
					// major, minor and patch segments match a version from the index, which is
					// where Helm could return a matching version based on those segments, but not
					// strictly equal to the specified one
					minVersion: "3.0.0+up1.2.3",
					// no exact version
				}: {},
				{
					namespace:    "cattle-system",
					name:         "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					name:         "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			takeOwnership: false,
			expectInstalls: map[string]bool{
				"fleet":           false,
				"rancher-webhook": true,
				"aks-operator":    false,
			},
			expectedErr: errors.New("specified version 3.0.0+up1.2.3 doesn't exist in the index"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			ctx := context.Background()
			mockContentClient := NewMockContentClient(ctrl)
			mockOperationClient := NewMockOperationClient(ctrl)
			mockPodClient := fake.NewMockClientInterface[*v1.Pod, *v1.PodList](ctrl)
			mockHelmClient := NewMockHelmClient(ctrl)

			for dc := range test.desiredCharts {
				var foundRelease *release.Release
				for _, r := range test.releases {
					if r.Name == dc.name && r.Namespace == dc.namespace {
						foundRelease = r
					}
				}

				var foundReleases []*release.Release
				if foundRelease != nil {
					foundReleases = []*release.Release{foundRelease}
				}

				// Call from installCharts and isInstalled
				mockHelmClient.EXPECT().ListReleases(dc.namespace, dc.name, action.ListDeployed).
					Return(foundReleases, nil).
					MaxTimes(2)

				if test.expectInstalls[dc.name] {
					// Call from install -> hasStatus
					mockHelmClient.EXPECT().ListReleases(
						dc.namespace,
						dc.name,
						action.ListPendingInstall|action.ListPendingUpgrade|action.ListPendingRollback,
					).
						Return(nil, nil)

					upgradeOp := catalog.Operation{
						Status: catalog.OperationStatus{
							PodNamespace: dc.namespace,
						},
					}

					mockOperationClient.EXPECT().Upgrade(ctx, installUser, "", "rancher-charts", gomock.Any(), gomock.Any()).
						Return(&upgradeOp, nil)

					pod := v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name: "helm",
									State: v1.ContainerState{
										Terminated: &v1.ContainerStateTerminated{
											ExitCode: 0,
										},
									},
								},
							},
						},
					}

					mockPodClient.EXPECT().Get(dc.namespace, gomock.Any(), metav1.GetOptions{}).
						Return(&pod, nil).Times(1)
				}

				mockContentClient.EXPECT().Index("", "rancher-charts", "", true).
					Return(&repo.IndexFile{Entries: test.indexedReleases}, nil)
			}

			manager := Manager{
				ctx:           context.Background(),
				content:       mockContentClient,
				pods:          mockPodClient,
				desiredCharts: test.desiredCharts,
				helmClient:    mockHelmClient,
				operation:     mockOperationClient,
			}

			err := manager.installCharts(test.desiredCharts, test.takeOwnership)
			if test.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), test.expectedErr.Error())
			}
		})
	}
}
