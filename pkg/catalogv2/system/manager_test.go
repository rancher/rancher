package system

//go:generate go tool -modfile ../../../gotools/mockgen/go.mod mockgen --build_flags=--mod=mod -package system -destination=./mock_system_test.go github.com/rancher/rancher/pkg/catalogv2/system ContentClient,OperationClient,HelmClient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/repo/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/workqueue"
)

func TestManagerRemovesRelease(t *testing.T) {
	t.Parallel()
	webhook := releaseKey{namespace: "cattle-system", releaseName: "rancher-webhook"}
	charts := map[releaseKey]desired{
		webhook: {
			key: desiredKey{
				namespace:            "cattle-system",
				chartName:            "rancher-webhook",
				releaseName:          "rancher-webhook",
				minVersion:           "1.1.1",
				exactVersion:         "1.2.0",
				installImageOverride: "some-image",
			},
			values: desiredValues{Values: map[string]any{"foo": "bar"}},
		},
	}

	manager := Manager{desiredCharts: charts}
	manager.Remove("hello", "world")
	assert.Equal(t, charts, manager.desiredCharts)

	manager.Remove("cattle-system", "rancher-webhook")
	assert.Equal(t, map[releaseKey]desired{}, manager.desiredCharts)
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
		settings:      mockSettingsController,
		clusterRepos:  mockClusterRepos,
		queue:         newQueue("test", retryMaxDelay),
		priorityQueue: newQueue("test-priority", priorityRetryMaxDelay),
	}

	manager.Start(ctx)
	manager.queue.ShutDown()
	manager.priorityQueue.ShutDown()
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
				Status: releasecommon.StatusDeployed,
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
				Status: releasecommon.StatusDeployed,
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
				Status: releasecommon.StatusDeployed,
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
				Status: releasecommon.StatusDeployed,
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
				Status: releasecommon.StatusDeployed,
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
				Status: releasecommon.StatusDeployed,
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
				Status: releasecommon.StatusDeployed,
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
		desiredCharts   map[desiredKey]desiredValues
		expectInstalls  map[string]bool
		expectedErrors  map[string]error
	}{
		{
			name:     "Updates charts to desired version",
			releases: []*release.Release{&rancherChartV1, &fleetChartV1, &aksOperatorChartV1},
			indexedReleases: map[string]repo.ChartVersions{
				"fleet":           {&fleetRepoV1, &fleetRepoV2},
				"rancher-webhook": {&rancherRepoV1, &rancherRepoV2},
				"aks-operator":    {&aksOperatorRepoV1, &aksOperatorRepoV2},
			},
			desiredCharts: map[desiredKey]desiredValues{
				{
					namespace:    "cattle-fleet-system",
					chartName:    "fleet",
					releaseName:  "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "rancher-webhook",
					releaseName:  "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "aks-operator",
					releaseName:  "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
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
			desiredCharts: map[desiredKey]desiredValues{
				{
					namespace:    "cattle-fleet-system",
					chartName:    "fleet",
					releaseName:  "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "rancher-webhook",
					releaseName:  "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "aks-operator",
					releaseName:  "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
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
			desiredCharts: map[desiredKey]desiredValues{
				{
					namespace:    "cattle-fleet-system",
					chartName:    "fleet",
					releaseName:  "fleet",
					minVersion:   "1.0.0",
					exactVersion: "",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "rancher-webhook",
					releaseName:  "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "aks-operator",
					releaseName:  "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
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
			desiredCharts: map[desiredKey]desiredValues{
				{
					namespace:    "cattle-fleet-system",
					chartName:    "fleet",
					releaseName:  "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "rancher-webhook",
					releaseName:  "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "aks-operator",
					releaseName:  "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
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
			desiredCharts: map[desiredKey]desiredValues{
				{
					namespace:    "cattle-fleet-system",
					chartName:    "fleet",
					releaseName:  "fleet",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "rancher-webhook",
					releaseName:  "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "aks-operator",
					releaseName:  "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
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
			desiredCharts: map[desiredKey]desiredValues{
				{
					namespace:   "cattle-fleet-system",
					chartName:   "fleet",
					releaseName: "fleet",
					// major, minor and patch segments match a version from the index, which is
					// where Helm could return a matching version based on those segments, but not
					// strictly equal to the specified one
					minVersion: "3.0.0+up1.2.3",
					// no exact version
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "rancher-webhook",
					releaseName:  "rancher-webhook",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
				{
					namespace:    "cattle-system",
					chartName:    "aks-operator",
					releaseName:  "aks-operator",
					minVersion:   "1.0.0",
					exactVersion: "2.0.0",
				}: {},
			},
			expectInstalls: map[string]bool{
				"fleet":           false,
				"rancher-webhook": true,
				"aks-operator":    false,
			},
			expectedErrors: map[string]error{
				"fleet":        errors.New("specified version 3.0.0+up1.2.3 doesn't exist in the index"),
				"aks-operator": errors.New("no chart version found for aks-operator-2.0.0"),
			},
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
					if r.Name == dc.releaseName && r.Namespace == dc.namespace {
						foundRelease = r
					}
				}

				var foundReleases []*release.Release
				if foundRelease != nil {
					foundReleases = []*release.Release{foundRelease}
				}
				// Call from installCharts and isInstalled
				mockHelmClient.EXPECT().ListReleases(dc.namespace, dc.releaseName, action.ListDeployed).
					Return(foundReleases, nil).
					MaxTimes(2)

				if test.expectInstalls[dc.chartName] {
					mockOperationClient.EXPECT().AddCpTaintsToTolerations([]v1.Toleration(nil)).Return([]v1.Toleration{{Value: "bar", Key: "foo"}}, nil)
					// Call from install -> hasStatus
					mockHelmClient.EXPECT().ListReleases(
						dc.namespace,
						dc.releaseName,
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
						Name: "foo",
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
				ctx:        context.Background(),
				content:    mockContentClient,
				pods:       mockPodClient,
				helmClient: mockHelmClient,
				operation:  mockOperationClient,
			}
			for chart, values := range test.desiredCharts {
				err := manager.installChart(chart, values)
				if test.expectedErrors != nil && test.expectedErrors[chart.chartName] != nil {
					assert.Contains(t, err.Error(), test.expectedErrors[chart.chartName].Error())
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

// newQueueTestManager returns a Manager whose installs are driven by listReleases. Returning deployedRelease makes the
// install a successful no-op, returning an error fails it. Chart versions are always "1.0.0".
func newQueueTestManager(t *testing.T, chartNames []string, listReleases func(namespace, name string) ([]*release.Release, error)) *Manager {
	ctrl := gomock.NewController(t)

	mockContentClient := NewMockContentClient(ctrl)
	mockContentClient.EXPECT().Index("", "rancher-charts", "", true).DoAndReturn(func(_, _, _ string, _ bool) (*repo.IndexFile, error) {
		entries := map[string]repo.ChartVersions{}
		for _, name := range chartNames {
			entries[name] = repo.ChartVersions{{Metadata: &chart.Metadata{Version: "1.0.0"}, URLs: []string{"foo"}}}
		}
		return &repo.IndexFile{Entries: entries}, nil
	}).AnyTimes()

	mockHelmClient := NewMockHelmClient(ctrl)
	mockHelmClient.EXPECT().ListReleases(gomock.Any(), gomock.Any(), action.ListDeployed).DoAndReturn(func(namespace, name string, _ action.ListStates) ([]*release.Release, error) {
		return listReleases(namespace, name)
	}).AnyTimes()

	m := &Manager{
		ctx:           context.Background(),
		content:       mockContentClient,
		helmClient:    mockHelmClient,
		desiredCharts: map[releaseKey]desired{},
		queue:         newQueue("test", retryMaxDelay),
		priorityQueue: newQueue("test-priority", priorityRetryMaxDelay),
	}
	// wait for in-flight installs before the mock controller is finished
	t.Cleanup(func() {
		m.queue.ShutDownWithDrain()
		m.priorityQueue.ShutDownWithDrain()
	})
	return m
}

func deployedRelease(namespace, name string) []*release.Release {
	return []*release.Release{{
		Namespace: namespace,
		Name:      name,
		Chart:     &chart.Chart{Metadata: &chart.Metadata{Version: "1.0.0"}},
		Info:      &release.Info{Status: releasecommon.StatusDeployed},
	}}
}

func ensureTestChart(m *Manager, namespace, name string, values map[string]any) {
	_ = m.Ensure(namespace, name, name, "", "1.0.0", values, false, "")
}

func TestPriorityChartNotBlockedByHungCharts(t *testing.T) {
	t.Parallel()

	var hungCharts []string
	for i := range defaultWorkers + 1 {
		hungCharts = append(hungCharts, fmt.Sprintf("hung-%d", i))
	}

	unblock := make(chan struct{})
	rdpInstalled := make(chan struct{})
	var once sync.Once
	m := newQueueTestManager(t, append(hungCharts, "remotedialer-proxy"), func(namespace, name string) ([]*release.Release, error) {
		if name == "remotedialer-proxy" {
			once.Do(func() { close(rdpInstalled) })
		} else {
			<-unblock
		}
		return deployedRelease(namespace, name), nil
	})
	t.Cleanup(func() { close(unblock) })

	for range priorityWorkers {
		go m.runWorker(m.priorityQueue)
	}
	for range defaultWorkers {
		go m.runWorker(m.queue)
	}

	// every default worker is stuck on a hung chart before the priority chart is requested
	for _, name := range hungCharts {
		ensureTestChart(m, "cattle-system", name, nil)
	}
	ensureTestChart(m, namespace.System, "remotedialer-proxy", nil)

	select {
	case <-rdpInstalled:
	case <-time.After(5 * time.Second):
		t.Fatal("remotedialer-proxy was not installed while other charts were hung")
	}
}

func TestFailedChartRetriedWithoutBlockingOthers(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	attempts := map[string]int{}
	m := newQueueTestManager(t, []string{"broken", "good"}, func(namespace, name string) ([]*release.Release, error) {
		mu.Lock()
		defer mu.Unlock()
		attempts[name]++
		if name == "broken" && attempts[name] == 1 {
			return nil, errors.New("transient failure")
		}
		return deployedRelease(namespace, name), nil
	})
	m.queue = workqueue.NewTypedRateLimitingQueue(workqueue.NewTypedItemExponentialFailureRateLimiter[releaseKey](time.Millisecond, time.Millisecond))

	ensureTestChart(m, "cattle-system", "broken", nil)
	ensureTestChart(m, "cattle-system", "good", nil)

	// broken fails and is requeued, good is installed next, then broken is retried and succeeds
	for range 3 {
		assert.True(t, m.processNextItem(m.queue))
	}

	assert.Equal(t, map[string]int{"broken": 2, "good": 1}, attempts)
	assert.Equal(t, 0, m.queue.Len())
	assert.Equal(t, 0, m.queue.NumRequeues(releaseKey{namespace: "cattle-system", releaseName: "broken"}))
}

func TestRemovedChartIsNotInstalled(t *testing.T) {
	t.Parallel()

	m := newQueueTestManager(t, []string{"fleet"}, func(namespace, name string) ([]*release.Release, error) {
		t.Errorf("unexpected install of %s/%s", namespace, name)
		return nil, nil
	})

	ensureTestChart(m, "cattle-fleet-system", "fleet", nil)
	m.Remove("cattle-fleet-system", "fleet")

	assert.True(t, m.processNextItem(m.queue))
	assert.Equal(t, 0, m.queue.Len())
}

func TestEnsureQueuesOnlyOnChange(t *testing.T) {
	t.Parallel()

	m := newQueueTestManager(t, nil, func(namespace, name string) ([]*release.Release, error) {
		t.Errorf("unexpected install of %s/%s", namespace, name)
		return nil, nil
	})
	fleet := releaseKey{namespace: "cattle-fleet-system", releaseName: "fleet"}

	ensureTestChart(m, namespace.System, "remotedialer-proxy", nil)
	assert.Equal(t, 1, m.priorityQueue.Len())
	assert.Equal(t, 0, m.queue.Len())

	ensureTestChart(m, fleet.namespace, fleet.releaseName, map[string]any{"foo": "bar"})
	assert.Equal(t, 1, m.queue.Len())
	popQueue(m.queue)

	// unchanged
	ensureTestChart(m, fleet.namespace, fleet.releaseName, map[string]any{"foo": "bar"})
	assert.Equal(t, 0, m.queue.Len())

	// changed values
	ensureTestChart(m, fleet.namespace, fleet.releaseName, map[string]any{"foo": "baz"})
	assert.Equal(t, 1, m.queue.Len())
	popQueue(m.queue)

	// changed version replaces the existing entry for the release
	_ = m.Ensure(fleet.namespace, fleet.releaseName, fleet.releaseName, "", "2.0.0", map[string]any{"foo": "baz"}, false, "")
	assert.Equal(t, 1, m.queue.Len())
	assert.Len(t, m.desiredCharts, 2)
	assert.Equal(t, "2.0.0", m.desiredCharts[fleet].key.exactVersion)
}

// popQueue takes the next item off the queue without installing it.
func popQueue(queue workqueue.TypedRateLimitingInterface[releaseKey]) {
	key, _ := queue.Get()
	queue.Forget(key)
	queue.Done(key)
}

func TestRefreshTakesOwnership(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockContentClient := NewMockContentClient(ctrl)
	mockContentClient.EXPECT().Index("", "rancher-charts", "", true).DoAndReturn(func(_, _, _ string, _ bool) (*repo.IndexFile, error) {
		return &repo.IndexFile{Entries: map[string]repo.ChartVersions{
			"rancher-webhook": {{Metadata: &chart.Metadata{Version: "1.0.0"}, URLs: []string{"foo"}}},
		}}, nil
	}).AnyTimes()

	// never installed, so every attempt upgrades
	mockHelmClient := NewMockHelmClient(ctrl)
	mockHelmClient.EXPECT().ListReleases(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	var takeOwnership []bool
	mockOperationClient := NewMockOperationClient(ctrl)
	mockOperationClient.EXPECT().AddCpTaintsToTolerations(gomock.Any()).Return(nil, nil).AnyTimes()
	mockOperationClient.EXPECT().Upgrade(gomock.Any(), installUser, "", "rancher-charts", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ user.Info, _, _ string, options io.Reader, _ string) (*catalog.Operation, error) {
			var upgrade types.ChartUpgradeAction
			if err := json.NewDecoder(options).Decode(&upgrade); err != nil {
				return nil, err
			}
			takeOwnership = append(takeOwnership, upgrade.TakeOwnership)
			return &catalog.Operation{Status: catalog.OperationStatus{PodNamespace: "cattle-system", PodName: "helm-operation"}}, nil
		}).AnyTimes()

	mockPodClient := fake.NewMockClientInterface[*v1.Pod, *v1.PodList](ctrl)
	mockPodClient.EXPECT().Get("cattle-system", "helm-operation", metav1.GetOptions{}).Return(&v1.Pod{
		Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{{
			Name:  "helm",
			State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 0}},
		}}},
	}, nil).AnyTimes()

	m := &Manager{
		ctx:           context.Background(),
		content:       mockContentClient,
		helmClient:    mockHelmClient,
		operation:     mockOperationClient,
		pods:          mockPodClient,
		desiredCharts: map[releaseKey]desired{},
		queue:         newQueue("test", retryMaxDelay),
		priorityQueue: newQueue("test-priority", priorityRetryMaxDelay),
	}
	t.Cleanup(func() {
		m.queue.ShutDown()
		m.priorityQueue.ShutDown()
	})

	// the initial install uses the requested takeOwnership
	_ = m.Ensure("cattle-system", "rancher-webhook", "rancher-webhook", "", "1.0.0", map[string]any{"foo": "bar"}, false, "")
	assert.True(t, m.processNextItem(m.queue))
	assert.Equal(t, []bool{false}, takeOwnership)

	// refreshes take ownership
	m.enqueueAll()
	assert.True(t, m.processNextItem(m.queue))
	assert.Equal(t, []bool{false, true}, takeOwnership)

	// an unchanged Ensure keeps the chart a refresh
	_ = m.Ensure("cattle-system", "rancher-webhook", "rancher-webhook", "", "1.0.0", map[string]any{"foo": "bar"}, false, "")
	assert.Equal(t, 0, m.queue.Len())

	// changed values are installed with the requested takeOwnership again
	_ = m.Ensure("cattle-system", "rancher-webhook", "rancher-webhook", "", "1.0.0", map[string]any{"foo": "baz"}, false, "")
	assert.True(t, m.processNextItem(m.queue))
	assert.Equal(t, []bool{false, true, false}, takeOwnership)
}
