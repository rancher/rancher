package system

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

const testChartVersion = "1.0.0"

// staticIndex is a ContentClient serving an index that contains every chart these tests install,
// so that install() gets past the index lookup and on to the release check, which is where
// fakeHelm can see which chart is being installed.
type staticIndex struct {
	names []string
}

func (s *staticIndex) Index(_, _, _ string, _ bool) (*repo.IndexFile, error) {
	index := repo.NewIndexFile()
	for _, name := range s.names {
		index.Add(&chart.Metadata{
			APIVersion: "v2",
			Name:       name,
			Version:    testChartVersion,
		}, fmt.Sprintf("%s-%s.tgz", name, testChartVersion), "", "")
	}
	index.SortEntries()
	return index, nil
}

// fakeHelm observes and controls each install. install() calls ListReleases with the release
// name, so this is the first point in the install path that knows which chart is running.
//
// A release reported as already deployed at testChartVersion makes install() return nil without
// needing operation or pod mocks. Returning an error instead makes the install fail.
type fakeHelm struct {
	mu sync.Mutex
	// fail holds the release names whose installs should fail.
	fail map[string]error
	// hold is how long each install blocks for, so tests can observe overlap.
	hold time.Duration
	// starts receives every release name as its install begins.
	starts chan string
	calls  map[string]int
}

func newFakeHelm(hold time.Duration) *fakeHelm {
	return &fakeHelm{
		fail:   map[string]error{},
		hold:   hold,
		starts: make(chan string, 32),
		calls:  map[string]int{},
	}
}

func (f *fakeHelm) failWith(releaseName string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail[releaseName] = err
}

func (f *fakeHelm) succeed(releaseName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.fail, releaseName)
}

func (f *fakeHelm) callCount(releaseName string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[releaseName]
}

func (f *fakeHelm) ListReleases(namespace, name string, _ action.ListStates) ([]*release.Release, error) {
	f.mu.Lock()
	f.calls[name]++
	err, shouldFail := f.fail[name]
	hold := f.hold
	f.mu.Unlock()

	select {
	case f.starts <- name:
	default:
	}

	if hold > 0 {
		time.Sleep(hold)
	}
	if shouldFail {
		return nil, err
	}

	// Already deployed at the version the index offers, so install() is a no-op and returns nil.
	return []*release.Release{{
		Namespace: namespace,
		Name:      name,
		Chart:     &chart.Chart{Metadata: &chart.Metadata{Name: name, Version: testChartVersion}},
		Info:      &release.Info{Status: releasecommon.StatusDeployed},
		Config:    map[string]interface{}{},
	}}, nil
}

// awaitStart waits for the next install to begin and returns the chart it was for.
func awaitStart(t *testing.T, helm *fakeHelm, timeout time.Duration) string {
	t.Helper()
	select {
	case name := <-helm.starts:
		return name
	case <-time.After(timeout):
		t.Fatalf("no install started within %s", timeout)
		return ""
	}
}

func newSyncManager(t *testing.T, content ContentClient, helm HelmClient) *Manager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := &Manager{
		ctx:                   ctx,
		content:               content,
		helmClient:            helm,
		sync:                  make(chan desired, 10),
		desiredCharts:         map[desiredKey]map[string]interface{}{},
		refreshIntervalChange: make(chan struct{}, 1),
		trigger:               make(chan struct{}, 1),
	}
	go m.runSync()
	return m
}

func ensureChart(t *testing.T, m *Manager, chartName string) {
	t.Helper()
	require.NoError(t, m.Ensure("cattle-system", chartName, chartName, "", "", map[string]interface{}{}, false, ""))
}

// TestEnsureSendsInCallerOrder covers the requirement that rancher-webhook is queued first.
// Ensure used to push onto the sync channel from a goroutine per call, which randomised the order
// getChartsToInstall had deliberately chosen, so the webhook could end up behind the
// system-upgrade-controller's whole install.
func TestEnsureSendsInCallerOrder(t *testing.T) {
	// No runSync here: read the queue directly to observe the order Ensure produced.
	m := &Manager{
		ctx:  context.Background(),
		sync: make(chan desired, 10),
	}

	want := []string{webhookChartName, "system-upgrade-controller"}
	for _, chartName := range want {
		ensureChart(t, m, chartName)
	}

	var got []string
	for range want {
		select {
		case d := <-m.sync:
			got = append(got, d.key.chartName)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out reading the queue, saw %v", got)
		}
	}

	assert.Equal(t, want, got, "Ensure must queue charts in caller order so the webhook goes first")
}

// TestRunSyncInstallsConcurrently covers the serialization defect: installs used to run inline on
// the single runSync goroutine, and install() blocks until the helm operation finishes (the helm
// action sets Wait: true), so two independent charts took the sum of their durations.
//
// Both charts here are non-webhook charts. The webhook barrier deliberately serialises against
// rancher-webhook only, and this test exists to prove it did not serialise everything.
func TestRunSyncInstallsConcurrently(t *testing.T) {
	const hold = 3 * time.Second

	names := []string{"system-upgrade-controller", "fleet-crd"}
	helm := newFakeHelm(hold)
	m := newSyncManager(t, &staticIndex{names: names}, helm)

	for _, name := range names {
		ensureChart(t, m, name)
	}

	awaitStart(t, helm, 5*time.Second)

	// The second install must begin while the first is still running. Serialized, it could only
	// begin after the first finished, i.e. not before hold has elapsed.
	select {
	case <-helm.starts:
	case <-time.After(hold / 2):
		t.Fatal("second install did not start while the first was still running; installs are serialized")
	}
}

// TestRunSyncWebhookBarrier covers the ordering defect that broke provisioning: rancher-webhook
// serves the admission webhooks that validate the secrets every later chart creates, so a chart
// installed alongside it races the webhook becoming ready and fails with
// "failed calling webhook rancher.cattle.io.secrets". Nothing else may start while the webhook is
// installing.
func TestRunSyncWebhookBarrier(t *testing.T) {
	const hold = 2 * time.Second

	names := []string{webhookChartName, "system-upgrade-controller"}
	helm := newFakeHelm(hold)
	m := newSyncManager(t, &staticIndex{names: names}, helm)

	for _, name := range names {
		ensureChart(t, m, name)
	}

	assert.Equal(t, webhookChartName, awaitStart(t, helm, 5*time.Second), "the webhook must install first")

	// Nothing else may start while the webhook install is in flight.
	select {
	case name := <-helm.starts:
		t.Fatalf("%s started while the %s install was still in flight", name, webhookChartName)
	case <-time.After(hold / 2):
	}

	// Once the webhook finishes, the held chart is released rather than stranded.
	assert.Equal(t, "system-upgrade-controller", awaitStart(t, helm, 15*time.Second))
}

// TestRunSyncRetriesFailedInstall covers the regression that broke every imported-cluster test:
// system-upgrade-controller failed once because rancher-webhook was not serving yet, and was then
// never retried, so the systemagent controller never created the cluster's beacon. Every install
// failure has to be retried, not just a not-yet-built index.
func TestRunSyncRetriesFailedInstall(t *testing.T) {
	const name = "system-upgrade-controller"

	helm := newFakeHelm(0)
	// The exact error CI hit. It is not a NotFound and not ErrNoChartName, so the previous
	// classification treated it as permanent.
	helm.failWith(name, errors.New(`Internal error occurred: failed calling webhook "rancher.cattle.io.secrets"`))

	m := newSyncManager(t, &staticIndex{names: []string{name}}, helm)
	ensureChart(t, m, name)

	assert.Equal(t, name, awaitStart(t, helm, 5*time.Second), "first attempt")

	// The first backoff is installRetryBaseDelay and the retry tick runs every
	// installRetryPollInterval, so allow for both.
	timeout := installRetryBaseDelay + 2*installRetryPollInterval
	assert.Equal(t, name, awaitStart(t, helm, timeout), "the failed install was never retried")

	// Once it can succeed, the retry loop stops re-driving it.
	helm.succeed(name)
	awaitStart(t, helm, timeout)

	before := helm.callCount(name)
	require.Eventually(t, func() bool {
		_, installed := m.desiredValues(desiredKey{namespace: "cattle-system", chartName: name, releaseName: name})
		return installed
	}, 10*time.Second, 100*time.Millisecond, "a successful install must be recorded as desired")

	time.Sleep(installRetryBaseDelay + installRetryPollInterval)
	assert.LessOrEqual(t, helm.callCount(name), before+1, "the chart is still being retried after it succeeded")
}

// TestRunSyncRemoveDuringSyncIsRaceFree covers the crash: Remove is called from the systemcharts
// controller goroutine and used to delete from desiredCharts while runSync iterated it, which
// killed Rancher with "fatal error: concurrent map iteration and map write". Run with -race.
func TestRunSyncRemoveDuringSyncIsRaceFree(t *testing.T) {
	names := []string{webhookChartName, "system-upgrade-controller", "fleet-crd", "remotedialer-proxy"}

	helm := newFakeHelm(0)
	m := newSyncManager(t, &staticIndex{names: names}, helm)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Keep charts flowing in, so runSync keeps writing desiredCharts.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, name := range names {
				require.NoError(t, m.Ensure("cattle-system", name, name, "", "", map[string]interface{}{"n": time.Now().UnixNano()}, false, ""))
			}
		}
	}()

	// Keep the refresh path iterating desiredCharts.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			select {
			case m.trigger <- struct{}{}:
			default:
			}
		}
	}()

	// And hammer Remove from a third goroutine, as the systemcharts controller does.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, name := range names {
				m.Remove("cattle-system", name)
			}
		}
	}()

	time.Sleep(2 * time.Second)
	close(stop)
	wg.Wait()
}
