package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// pendingHelmReleaseSelector matches helm release secrets left mid-operation.
	pendingHelmReleaseSelector = "owner=helm,status in (pending-install,pending-upgrade,pending-rollback)"

	// systemChartsSettleTimeout covers the clone of rancher-charts plus one 5 minute ClusterRepo retry backoff.
	systemChartsSettleTimeout = 10 * time.Minute
	systemChartsSettlePoll    = 30 * time.Second
)

// Kubectl runs kubectl against the downstream cluster with args and returns its stdout.
type Kubectl func(ctx context.Context, args ...string) ([]byte, error)

// getRaw GETs a server relative URI from the downstream API server.
type getRaw func(ctx context.Context, uri string) ([]byte, error)

// WaitForSystemChartsSettled blocks until the downstream rancher-charts ClusterRepo has a usable index
// and no helm release is mid-operation, so an etcd snapshot taken afterwards can't capture either.
//
// A snapshot taken while rancher-charts is cloning restores a ClusterRepo with no index whose status
// makes the repo handler skip downloading for its 1h refresh interval, so no system chart can be
// installed. A snapshot taken mid helm operation restores a pending release, which the system charts
// manager treats as in progress and leaves alone.
func WaitForSystemChartsSettled(t *testing.T, kube kubernetes.Interface) {
	t.Helper()
	t.Logf("waiting for system charts to settle before taking etcd snapshot. Will wait for up to %s", systemChartsSettleTimeout.String())
	waitForSystemChartsSettled(t, func(ctx context.Context, uri string) ([]byte, error) {
		return kube.Discovery().RESTClient().Get().RequestURI(uri).SetHeader("Accept", "application/json").DoRaw(ctx)
	})
}

// WaitForSystemChartsSettledKubectl is WaitForSystemChartsSettled for clusters only reachable by
// running kubectl on one of their nodes.
func WaitForSystemChartsSettledKubectl(t *testing.T, kubectl Kubectl) {
	t.Helper()
	waitForSystemChartsSettled(t, func(ctx context.Context, uri string) ([]byte, error) {
		return kubectl(ctx, "get", "--raw", uri)
	})
}

func waitForSystemChartsSettled(t *testing.T, get getRaw) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), systemChartsSettleTimeout)
	defer cancel()

	start := time.Now()
	var reason string
	err := wait.PollUntilContextCancel(ctx, systemChartsSettlePoll, true, func(ctx context.Context) (bool, error) {
		reason = systemChartsUnsettledReason(ctx, get)
		if reason != "" {
			t.Log("system charts not settled before etcd snapshot, reason: " + reason)
		}
		return reason == "", nil
	})
	if err != nil {
		t.Fatalf("system charts did not settle before etcd snapshot within %s: %v: %s", systemChartsSettleTimeout, err, reason)
	}
	t.Logf("system charts settled before etcd snapshot after %s", time.Since(start).Round(time.Second))
}

// systemChartsUnsettledReason returns why the downstream cluster is not safe to snapshot, or "" if it is.
func systemChartsUnsettledReason(ctx context.Context, get getRaw) string {
	repo := &catalogv1.ClusterRepo{}
	if err := getJSON(ctx, get, "/apis/catalog.cattle.io/v1/clusterrepos/rancher-charts", repo); err != nil {
		return err.Error()
	}

	downloaded := condition.Cond(catalogv1.RepoDownloaded)
	switch {
	case repo.Status.ObservedGeneration != repo.Generation:
		return fmt.Sprintf("clusterrepo rancher-charts observedGeneration %d does not match generation %d", repo.Status.ObservedGeneration, repo.Generation)
	case !downloaded.IsTrue(repo):
		return fmt.Sprintf("clusterrepo rancher-charts is not downloaded: %s", downloaded.GetMessage(repo))
	case repo.Status.IndexConfigMapName == "":
		return "clusterrepo rancher-charts has no index configmap"
	case repo.Status.ShouldNotSkip || repo.Status.NumberOfRetries > 0:
		return fmt.Sprintf("clusterrepo rancher-charts has a reconcile pending (shouldNotSkip=%t, numberOfRetries=%d)", repo.Status.ShouldNotSkip, repo.Status.NumberOfRetries)
	}

	cm := fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", repo.Status.IndexConfigMapNamespace, repo.Status.IndexConfigMapName)
	if _, err := get(ctx, cm); err != nil {
		return fmt.Sprintf("getting clusterrepo rancher-charts index configmap: %v", err)
	}

	secrets := &corev1.SecretList{}
	if err := getJSON(ctx, get, "/api/v1/secrets?"+url.Values{"labelSelector": {pendingHelmReleaseSelector}}.Encode(), secrets); err != nil {
		return err.Error()
	}
	if len(secrets.Items) > 0 {
		var pending []string
		for _, s := range secrets.Items {
			pending = append(pending, fmt.Sprintf("%s/%s (%s)", s.Namespace, s.Labels["name"], s.Labels["status"]))
		}
		return fmt.Sprintf("helm releases are pending: %v", pending)
	}
	return ""
}

func getJSON(ctx context.Context, get getRaw, uri string, into any) error {
	data, err := get(ctx, uri)
	if err != nil {
		return fmt.Errorf("getting %s: %w", uri, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("decoding %s: %w", uri, err)
	}
	return nil
}
