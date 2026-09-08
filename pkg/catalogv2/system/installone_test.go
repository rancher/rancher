package system

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/rancher/pkg/catalogv2/system/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestInstallOneReturnsMissingIndexError covers the contract runSync relies on to retry: on a
// fresh cluster the rancher-charts index ConfigMap does not exist yet, and installOne has to
// report that rather than swallow it.
//
// It also must not block. An earlier implementation slept and retried in a loop, which stalled
// every other chart, the refresh ticker and any pending Ensure for as long as the index was
// missing.
func TestInstallOneReturnsMissingIndexError(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "rancher-charts-index")

	contentMock := &mocks.ContentClient{}
	contentMock.On("Index", "", systemChartsRepoName, "", true).Return(nil, notFound)

	m := &Manager{ctx: context.Background(), content: contentMock}

	key := desiredKey{namespace: "cattle-system", chartName: webhookChartName, releaseName: webhookChartName}
	err := m.installOne(key, map[string]interface{}{}, true)

	require.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err), "the NotFound must survive so isIndexNotReady can classify it")
	assert.True(t, isIndexNotReady(err))
}

// TestIsIndexNotReady pins the classification. It only selects the log level now — every failure
// is retried — but the ErrNoChartName case has to keep matching through errors.Is, which means
// install must return the sentinel rather than a flattened copy of its message.
func TestIsIndexNotReady(t *testing.T) {
	assert.True(t, isIndexNotReady(repo.ErrNoChartName))
	assert.True(t, isIndexNotReady(apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "x")))
	assert.False(t, isIndexNotReady(errors.New("failed calling webhook \"rancher.cattle.io.secrets\"")))
}

// TestInstallBackoff covers the retry schedule: it doubles and then holds at the cap, so a chart
// that cannot be installed is still retried forever without spamming the API server.
func TestInstallBackoff(t *testing.T) {
	assert.Equal(t, installRetryBaseDelay, installBackoff(0), "a zero attempt count is treated as the first attempt")
	assert.Equal(t, installRetryBaseDelay, installBackoff(1))
	assert.Equal(t, 2*installRetryBaseDelay, installBackoff(2))
	assert.Equal(t, 4*installRetryBaseDelay, installBackoff(3))
	assert.Equal(t, maxInstallRetryDelay, installBackoff(100))

	// Monotonic and never past the cap.
	for i := 1; i < 50; i++ {
		assert.LessOrEqual(t, installBackoff(i), maxInstallRetryDelay)
		assert.LessOrEqual(t, installBackoff(i), installBackoff(i+1))
	}
}
