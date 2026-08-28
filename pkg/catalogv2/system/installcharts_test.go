package system

import (
	"context"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/catalogv2/system/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestInstallChartsDoesNotBlockOnMissingIndex covers the starvation regression.
//
// installCharts runs on the single runSync goroutine. On a fresh cluster the rancher-charts
// index ConfigMap does not exist yet, so content.Index returns a k8s NotFound. The previous
// implementation slept 5s and retried forever inside the loop, which stalled every other
// chart, the refresh ticker, and any pending Ensure calls for as long as the index was
// missing. It must now return promptly and schedule a retry instead.
func TestInstallChartsDoesNotBlockOnMissingIndex(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rancher-charts-index")

	contentMock := &mocks.ContentClient{}
	contentMock.On("Index", "", systemChartsRepoName, "", true).Return(nil, notFound)

	clusterRepoMock := &mocks.ClusterRepoController{}
	clusterRepoMock.On("EnqueueAfter", systemChartsRepoName, chartNotInIndexRetryDelay).Return()

	m := &Manager{
		ctx:          context.Background(),
		content:      contentMock,
		clusterRepos: clusterRepoMock,
	}

	charts := map[desiredKey]map[string]interface{}{
		{namespace: "cattle-system", chartName: "rancher-webhook", releaseName: "rancher-webhook"}:                                 {},
		{namespace: "cattle-system", chartName: "system-upgrade-controller", releaseName: "mcb-managed-system-upgrade-controller"}: {},
	}

	done := make(chan error, 1)
	go func() { done <- m.installCharts(charts, true) }()

	select {
	case err := <-done:
		// Both charts are reported, i.e. the second was attempted rather than starved
		// behind the first.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	case <-time.After(5 * time.Second):
		t.Fatal("installCharts blocked while the rancher-charts index was missing")
	}

	// A retry is scheduled so the charts are not stranded until the next ClusterRepo event.
	clusterRepoMock.AssertCalled(t, "EnqueueAfter", systemChartsRepoName, chartNotInIndexRetryDelay)
	contentMock.AssertNumberOfCalls(t, "Index", len(charts))
}

// TestInstallChartsToleratesNilClusterRepos guards the retry path against a Manager built
// without a ClusterRepo controller, as several existing tests do.
func TestInstallChartsToleratesNilClusterRepos(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rancher-charts-index")

	contentMock := &mocks.ContentClient{}
	contentMock.On("Index", "", systemChartsRepoName, "", true).Return(nil, notFound)

	m := &Manager{ctx: context.Background(), content: contentMock}

	charts := map[desiredKey]map[string]interface{}{
		{namespace: "cattle-system", chartName: "rancher-webhook", releaseName: "rancher-webhook"}: {},
	}

	assert.Error(t, m.installCharts(charts, true))
}
