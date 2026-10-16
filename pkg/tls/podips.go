package tls

import (
	"context"
	"net"
	"os"
	"sync/atomic"

	"github.com/rancher/rancher/pkg/namespace"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// imperativeAPIAppSelectorEnvVar carries the value of the "app" label on the
// Rancher server pods. The Helm chart templates it from the release fullname
// (e.g. "RELEASE-NAME-rancher"), which is what the pods are actually labeled
// with -- not the literal "rancher". See chart/templates/deployment.yaml.
// pkg/ext reads the same var (via os.Getenv, unvalidated, so it may be
// empty); there is no settings accessor for it, so we read it directly too.
const imperativeAPIAppSelectorEnvVar = "IMPERATIVE_API_APP_SELECTOR"

// podIPTracker watches pods matching the given label selector in a single
// namespace and maintains a snapshot of their current pod IPs. The snapshot
// is consumed by filterExistingCN to keep only live pod IPs on the
// dynamiclistener-managed cert. Non-IP CNs (hostnames) are never permitted
// here: the static default SANs (localhost, cluster IP, node IPs, etc.) are
// already allowed upstream via dynamiclistener's allowDefaultSANs wrapper
// before this filter ever runs, so anything reaching filterExistingCN is, by
// definition, not one of the intended SANs.
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
// as a dynamiclistener.Config.FilterExistingCN: only CNs that are live pod
// IPs pass through. Non-IP CNs (hostnames) are always rejected — see
// filterExistingCN and the podIPTracker doc comment for why.
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
	// Pre-sync: keep IP CNs unconditionally to avoid pruning legitimate
	// ones before we know what the live pod set looks like. Hostnames are
	// still rejected even pre-sync — that decision never depends on the
	// pod-IP snapshot, only on dynamiclistener's own allowDefaultSANs
	// short-circuit having already accepted the legitimate ones upstream.
	if v == nil {
		out := make([]string, 0, len(cns))
		for _, cn := range cns {
			if net.ParseIP(cn) != nil {
				out = append(out, cn)
			}
		}
		return out
	}
	ips := *v.(*map[string]struct{})
	out := make([]string, 0, len(cns))
	for _, cn := range cns {
		// Reject any non-IP CN outright. Hostnames beyond the static
		// default SANs (already handled upstream, before this filter is
		// ever consulted) have no legitimate path onto this cert — a
		// client that can reach the listener and control TLS SNI or the
		// HTTP Host header must never be able to add arbitrary hostnames.
		if net.ParseIP(cn) == nil {
			continue
		}
		if _, ok := ips[cn]; ok {
			out = append(out, cn)
		}
	}
	return out
}

// newRancherPodIPFilter wires a podIPTracker to the upstream Rancher
// server's pods (the app=<fullname> label in cattle-system, resolved by
// rancherPodSelector) and returns its FilterExistingCN closure. handlerName
// distinguishes each listener's tracker instance (e.g. for metrics/logging)
// since this is called once per listener (:443 and :444).
func newRancherPodIPFilter(ctx context.Context, pods corev1controllers.PodController, handlerName string) func(...string) []string {
	return newPodIPTracker(ctx, namespace.System, rancherPodSelector(), pods, handlerName)
}

// rancherPodSelector returns the label selector used to find the Rancher
// server pods whose IPs are allowed on the dynamiclistener-managed cert.
//
// All replicas must resolve this to the SAME value: each pod runs its own
// tracker over a shared cert secret, so a selector mismatch makes replicas
// disagree on the valid IP set and fight over the secret, causing perpetual
// cert churn.
//
// The source of truth is IMPERATIVE_API_APP_SELECTOR, which the chart
// templates from the release fullname specifically as the pod "app" label
// value (see chart/templates/deployment.yaml) and which pkg/ext already
// treats as the canonical selector. When unset -- unit tests and non-Helm
// installs where pods carry the plain label -- we fall back to the literal
// "app=rancher".
//
// We deliberately do NOT fall back to the peer-service setting
// (settings.PeerServices / CATTLE_PEER_SERVICE): despite today coinciding
// with the fullname, it holds a comma-separated, order-insensitive set of
// *Service names* -- not pod "app" label values -- so inferring a single
// "app=<name>" selector from it is unsound (a Service may select pods on any
// labels, and "first entry" has no defined meaning). Nor do we honor the RDP
// "api-extension" override: that selector targets the imperative-api-extension
// Service, not the Rancher server pods this filter is scoped to.
func rancherPodSelector() string {
	if v := os.Getenv(imperativeAPIAppSelectorEnvVar); v != "" {
		return "app=" + v
	}
	return "app=rancher"
}
