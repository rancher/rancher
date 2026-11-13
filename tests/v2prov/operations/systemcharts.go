package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// pendingHelmReleaseSelector matches helm release secrets left mid-operation.
	pendingHelmReleaseSelector = "owner=helm,status in (pending-install,pending-upgrade,pending-rollback)"

	// systemChartNamespacePrefix matches the namespaces the system charts manager installs releases into.
	systemChartNamespacePrefix = "cattle-"

	// stuckHelmReleaseAge is well past the 5 minute default helm operation timeout
	// (system-managed-charts-operation-timeout), after which helm marks a release failed rather than pending.
	stuckHelmReleaseAge = 10 * time.Minute

	// systemChartsSettleTimeout covers the initial clone of rancher-charts plus one 5 minute ClusterRepo retry backoff.
	systemChartsSettleTimeout = 20 * time.Minute
	systemChartsSettlePoll    = 30 * time.Second
)

// Kubectl runs kubectl against the downstream cluster with args and returns its stdout.
type Kubectl func(ctx context.Context, args ...string) ([]byte, error)

// getRaw GETs a server relative URI from the downstream API server.
type getRaw func(ctx context.Context, uri string) ([]byte, error)

// WaitForSystemChartsSettled blocks until the downstream rancher-charts ClusterRepo has a usable index
// and no system chart release is mid-operation, so an etcd snapshot taken afterwards can't capture either.
//
// A snapshot taken while rancher-charts is cloning restores a ClusterRepo with no index whose status
// makes the repo handler skip downloading for its 1h refresh interval, so no system chart can be
// installed. A snapshot taken mid helm operation restores a pending release, which the system charts
// manager treats as in progress and leaves alone.
func WaitForSystemChartsSettled(t *testing.T, kube kubernetes.Interface) {
	t.Helper()
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
	t.Logf("waiting for system charts to settle before taking etcd snapshot. Will wait for up to %s", systemChartsSettleTimeout)

	ctx, cancel := context.WithTimeout(t.Context(), systemChartsSettleTimeout)
	defer cancel()

	start := time.Now()
	var reason string
	err := wait.PollUntilContextCancel(ctx, systemChartsSettlePoll, true, func(ctx context.Context) (bool, error) {
		var err error
		reason, err = systemChartsUnsettledReason(ctx, get)
		if err != nil {
			reason = err.Error()
			return false, err
		}
		if reason != "" {
			t.Log("system charts not settled before etcd snapshot, reason: " + reason)
		}
		return reason == "", nil
	})
	if err != nil {
		t.Fatalf("system charts did not settle before etcd snapshot after %s: %s", time.Since(start).Round(time.Second), reason)
	}
	t.Logf("system charts settled before etcd snapshot after %s", time.Since(start).Round(time.Second))
}

// systemChartsUnsettledReason returns why the downstream cluster is not safe to snapshot, or "" if it is.
// It returns an error only when the cluster can't settle on its own.
func systemChartsUnsettledReason(ctx context.Context, get getRaw) (string, error) {
	if reason := rancherChartsIndexMissingReason(ctx, get); reason != "" {
		return reason, nil
	}
	return pendingSystemChartReleases(ctx, get)
}

// rancherChartsIndexMissingReason returns why rancher-charts has no usable index, or "" if it has one.
//
// Only a missing index is unsafe to snapshot. Once an index exists, a failed refresh (Downloaded=False,
// numberOfRetries > 0) or a pending follow-up reconcile (shouldNotSkip) leaves it in place, and a restored
// copy of that status is reconciled right away, so system charts can still install from it. Waiting on
// those instead can take longer than the timeout: retries back off 5m, ~10m, then ~20m, followed by the
// 1h refresh interval.
func rancherChartsIndexMissingReason(ctx context.Context, get getRaw) string {
	repo := &catalogv1.ClusterRepo{}
	if err := getJSON(ctx, get, "/apis/catalog.cattle.io/v1/clusterrepos/rancher-charts", repo); err != nil {
		return err.Error()
	}

	if repo.Status.IndexConfigMapName == "" {
		return fmt.Sprintf("clusterrepo rancher-charts has no index configmap yet (downloaded: %s)", condition.Cond(catalogv1.RepoDownloaded).GetMessage(repo))
	}

	cm := fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", repo.Status.IndexConfigMapNamespace, repo.Status.IndexConfigMapName)
	if _, err := get(ctx, cm); err != nil {
		return fmt.Sprintf("getting clusterrepo rancher-charts index configmap: %v", err)
	}
	return ""
}

// pendingSystemChartReleases returns which system chart releases are mid-operation, or "" if none are.
//
// Like the system charts manager, it only considers the latest revision of each release, and only releases
// in the namespaces the manager installs into, so distro charts (kube-system) and superseded revisions are
// ignored. The manager never recovers a release whose latest revision is stuck pending, so one that has been
// pending longer than stuckHelmReleaseAge is returned as an error instead of being waited on.
func pendingSystemChartReleases(ctx context.Context, get getRaw) (string, error) {
	secrets := &corev1.SecretList{}
	if err := getJSON(ctx, get, "/api/v1/secrets?"+url.Values{"labelSelector": {pendingHelmReleaseSelector}}.Encode(), secrets); err != nil {
		return err.Error(), nil
	}

	var pending []string
	for _, s := range secrets.Items {
		if !strings.HasPrefix(s.Namespace, systemChartNamespacePrefix) {
			continue
		}
		release := fmt.Sprintf("%s/%s (%s, revision %s)", s.Namespace, s.Labels["name"], s.Labels["status"], s.Labels["version"])

		latest, err := isLatestHelmRevision(ctx, get, s)
		if err != nil {
			return fmt.Sprintf("checking latest revision of helm release %s: %v", release, err), nil
		}
		if !latest {
			continue
		}

		if age := time.Since(s.CreationTimestamp.Time); age > stuckHelmReleaseAge {
			return "", fmt.Errorf("helm release %s has been pending for %s, and the system charts manager does not recover pending releases", release, age.Round(time.Second))
		}
		pending = append(pending, release)
	}

	if len(pending) > 0 {
		return fmt.Sprintf("helm releases are pending: %v", pending), nil
	}
	return "", nil
}

// isLatestHelmRevision reports whether the helm release secret s holds the newest revision of its release.
func isLatestHelmRevision(ctx context.Context, get getRaw, s corev1.Secret) (bool, error) {
	version, err := strconv.Atoi(s.Labels["version"])
	if err != nil {
		return false, fmt.Errorf("parsing version label %q of secret %s/%s: %w", s.Labels["version"], s.Namespace, s.Name, err)
	}

	revisions := &metav1.PartialObjectMetadataList{}
	selector := url.Values{"labelSelector": {"owner=helm,name=" + s.Labels["name"]}}.Encode()
	if err := getJSON(ctx, get, fmt.Sprintf("/api/v1/namespaces/%s/secrets?%s", s.Namespace, selector), revisions); err != nil {
		return false, err
	}

	for _, r := range revisions.Items {
		if v, err := strconv.Atoi(r.Labels["version"]); err == nil && v > version {
			return false, nil
		}
	}
	return true, nil
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
