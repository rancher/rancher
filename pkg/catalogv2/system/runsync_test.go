package system

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/catalogv2/system/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// countingContent is a ContentClient that signals each time install() reaches the index lookup
// and optionally holds the call, to simulate a slow install. It then returns a retriable error,
// since these tests are about how installs are scheduled, not about installing.
type countingContent struct {
	mu      sync.Mutex
	calls   int
	hold    time.Duration
	started chan struct{}
}

func (c *countingContent) Index(_, _, _ string, _ bool) (*repo.IndexFile, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()

	c.started <- struct{}{}
	if c.hold > 0 {
		time.Sleep(c.hold)
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rancher-charts-index")
}

func (c *countingContent) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func newSyncManager(t *testing.T, content ContentClient) *Manager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clusterRepos := &mocks.ClusterRepoController{}
	clusterRepos.On("EnqueueAfter", systemChartsRepoName, chartNotInIndexRetryDelay).Return().Maybe()

	m := &Manager{
		ctx:                   ctx,
		content:               content,
		sync:                  make(chan desired, 10),
		desiredCharts:         map[desiredKey]map[string]interface{}{},
		refreshIntervalChange: make(chan struct{}, 1),
		trigger:               make(chan struct{}, 1),
		clusterRepos:          clusterRepos,
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

	want := []string{"rancher-webhook", "system-upgrade-controller"}
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
func TestRunSyncInstallsConcurrently(t *testing.T) {
	const hold = 3 * time.Second

	content := &countingContent{hold: hold, started: make(chan struct{}, 4)}
	m := newSyncManager(t, content)

	ensureChart(t, m, "rancher-webhook")
	ensureChart(t, m, "system-upgrade-controller")

	select {
	case <-content.started:
	case <-time.After(5 * time.Second):
		t.Fatal("first install never started")
	}

	// The second install must begin while the first is still running. Serialized, it could only
	// begin after the first finished, i.e. not before hold has elapsed.
	select {
	case <-content.started:
	case <-time.After(hold / 2):
		t.Fatal("second install did not start while the first was still running; installs are serialized")
	}

	assert.Equal(t, 2, content.count())
}

// TestRunSyncSkipsDuplicateInFlight guards the inFlight bookkeeping: repeated Ensure calls for a
// key that is already installing must not start a second concurrent install of the same release.
func TestRunSyncSkipsDuplicateInFlight(t *testing.T) {
	content := &countingContent{hold: 3 * time.Second, started: make(chan struct{}, 4)}
	m := newSyncManager(t, content)

	for range 3 {
		ensureChart(t, m, "rancher-webhook")
	}

	select {
	case <-content.started:
	case <-time.After(10 * time.Second):
		t.Fatal("install never started")
	}

	select {
	case <-content.started:
		t.Fatal("a second concurrent install started for the same chart")
	case <-time.After(1 * time.Second):
	}
}
