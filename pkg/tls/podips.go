package tls

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/rancher/rancher/pkg/namespace"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// podIPTracker watches pods matching the given label selector in a single
// namespace and maintains a snapshot of their current pod IPs. The snapshot
// is consumed by filterExistingCN to prune stale IP CNs from the
// dynamiclistener-managed cert without disturbing other (hostname) CNs.
type podIPTracker struct {
	// ips holds *map[string]struct{}. nil before the first list completes:
	// in that pre-sync state filterExistingCN keeps everything (so we never
	// prematurely prune the cert at startup).
	ips atomic.Value

	namespace     string
	labelSelector string
	pods          corev1controllers.PodController
}

// newPodIPTracker registers an OnChange handler that updates the IP set on
// every relevant pod event. The tracker function it returns is safe to use
// as a dynamiclistener.Config.FilterExistingCN: non-IP CNs (hostnames) pass
// through unchanged, IP CNs are kept only if they match a currently-running
// pod IP.
func newPodIPTracker(ctx context.Context, ns, labelSelector string, pods corev1controllers.PodController, handlerName string) func(...string) []string {
	t := &podIPTracker{
		namespace:     ns,
		labelSelector: labelSelector,
		pods:          pods,
	}
	pods.OnChange(ctx, handlerName, t.onChange)
	return t.filterExistingCN
}

func (t *podIPTracker) onChange(_ string, pod *corev1.Pod) (*corev1.Pod, error) {
	// Skip pods in other namespaces. On delete events pod is nil; in that
	// case we proceed and let the List below return the correct live set.
	if pod != nil && pod.Namespace != t.namespace {
		return pod, nil
	}
	// On every event in our namespace (add/update/delete) re-list and
	// rebuild the snapshot. Cheap: there are only a handful of rancher pods.
	list, err := t.pods.List(t.namespace, metav1.ListOptions{LabelSelector: t.labelSelector})
	if err != nil {
		return pod, err
	}
	ips := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		if p.DeletionTimestamp != nil {
			continue
		}
		if p.Status.PodIP != "" {
			ips[p.Status.PodIP] = struct{}{}
		}
	}
	t.ips.Store(&ips)
	return pod, nil
}

func (t *podIPTracker) filterExistingCN(cns ...string) []string {
	v := t.ips.Load()
	// Pre-sync: keep everything to avoid pruning legitimate CNs before we
	// know what the live pod set looks like.
	if v == nil {
		return cns
	}
	ips := *v.(*map[string]struct{})
	out := make([]string, 0, len(cns))
	for _, cn := range cns {
		// Only prune IP CNs we can positively classify as stale.
		// Hostnames and any other non-IP CNs pass through — we don't have
		// enough information here to decide whether they're stale.
		if net.ParseIP(cn) == nil {
			out = append(out, cn)
			continue
		}
		if _, ok := ips[cn]; ok {
			out = append(out, cn)
		}
	}
	return out
}

// newRancherPodIPFilter wires a podIPTracker to the upstream Rancher
// server's pods (app=rancher in cattle-system) and returns its
// FilterExistingCN closure.
func newRancherPodIPFilter(ctx context.Context, pods corev1controllers.PodController) func(...string) []string {
	return newPodIPTracker(ctx, namespace.System, "app=rancher", pods, "rancher-podip-tls-internal-filter")
}
