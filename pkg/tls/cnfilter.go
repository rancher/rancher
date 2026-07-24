package tls

import (
	"context"
	"net"
	"strings"
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

// parseAllowedServices parses the comma-separated
// settings.TLSInternalCNAllowedServices value into a list of Service names
// in the cattle-system namespace (the only namespace Rancher's own pods can
// ever live in, so allowed Services are always assumed to live there too).
// Blank/whitespace-only entries are ignored.
func parseAllowedServices(value string) []string {
	var names []string
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		names = append(names, entry)
	}
	return names
}

// serviceIPTracker watches a fixed, admin-configured list of Services in
// cattle-system and maintains a snapshot of their current IPs (ClusterIP,
// ExternalIPs, and any LoadBalancer ingress IPs). Unlike podIPTracker, this
// is an *allowlist*: any CN matching an IP in the snapshot is always kept,
// regardless of what other filters would otherwise decide. This lets an
// admin explicitly trust a Service's IP (e.g. a LoadBalancer/VIP fronting
// Rancher) that the pod-IP filter has no way to recognize as legitimate,
// without disabling pruning entirely (which is what the
// listener.cattle.io/static workaround does).
type serviceIPTracker struct {
	// ips holds *map[string]struct{}, always non-nil after construction.
	ips atomic.Value

	names    []string
	services corev1controllers.ServiceController
}

// newServiceIPTracker registers an OnChange handler on Services in
// cattle-system that rebuilds the allowed-IP snapshot whenever a watched
// Service changes. If names is empty, the returned filter is a
// pass-through no-op (nothing extra is allowed).
func newServiceIPTracker(ctx context.Context, names []string, services corev1controllers.ServiceController, handlerName string) func(...string) []string {
	t := &serviceIPTracker{
		names:    names,
		services: services,
	}
	empty := map[string]struct{}{}
	t.ips.Store(&empty)

	if len(names) == 0 {
		return t.allowCN
	}

	services.OnChange(ctx, handlerName, t.onChange)
	// Populate synchronously with whatever is available right away, so we
	// don't need to wait for the informer to deliver its first events
	// before the allowlist is usable.
	t.rebuild()
	return t.allowCN
}

func (t *serviceIPTracker) watched(name string) bool {
	for _, n := range t.names {
		if n == name {
			return true
		}
	}
	return false
}

func (t *serviceIPTracker) onChange(_ string, svc *corev1.Service) (*corev1.Service, error) {
	// On delete events svc is nil; either way, only react to Services we
	// actually care about, then rebuild the whole snapshot from the live
	// set (cheap: at most a handful of watched Services).
	if svc != nil && !t.watched(svc.Name) {
		return svc, nil
	}
	t.rebuild()
	return svc, nil
}

func (t *serviceIPTracker) rebuild() {
	ips := make(map[string]struct{})
	for _, name := range t.names {
		svc, err := t.services.Get(namespace.System, name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != corev1.ClusterIPNone {
			ips[svc.Spec.ClusterIP] = struct{}{}
		}
		for _, ip := range svc.Spec.ExternalIPs {
			if ip != "" {
				ips[ip] = struct{}{}
			}
		}
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				ips[ingress.IP] = struct{}{}
			}
		}
	}
	t.ips.Store(&ips)
}

// allowCN keeps only the CNs that match a currently-known IP from the
// watched Services. Used to union this allowlist with another FilterCN via
// unionFilterCN.
func (t *serviceIPTracker) allowCN(cns ...string) []string {
	v := t.ips.Load()
	ips := *v.(*map[string]struct{})
	out := make([]string, 0, len(cns))
	for _, cn := range cns {
		if _, ok := ips[cn]; ok {
			out = append(out, cn)
		}
	}
	return out
}

// unionFilterCN returns a FilterCN closure that keeps a CN if either primary
// or allowlist accepts it. CNs rejected by primary are re-offered to
// allowlist rather than dropped outright, so an admin-approved Service IP is
// never pruned just because it isn't a live rancher pod IP.
func unionFilterCN(primary, allowlist func(...string) []string) func(...string) []string {
	return func(cns ...string) []string {
		kept := primary(cns...)
		if len(kept) == len(cns) {
			return kept
		}
		keptSet := make(map[string]struct{}, len(kept))
		for _, cn := range kept {
			keptSet[cn] = struct{}{}
		}
		var rejected []string
		for _, cn := range cns {
			if _, ok := keptSet[cn]; !ok {
				rejected = append(rejected, cn)
			}
		}
		if len(rejected) == 0 {
			return kept
		}
		return append(kept, allowlist(rejected...)...)
	}
}

// newRancherPodIPFilter wires a podIPTracker to the upstream Rancher
// server's pods (app=rancher in cattle-system) and returns its
// FilterExistingCN closure.
func newRancherPodIPFilter(ctx context.Context, pods corev1controllers.PodController) func(...string) []string {
	return newPodIPTracker(ctx, namespace.System, "app=rancher", pods, "rancher-podip-tls-internal-filter")
}

// newRancherInternalCNFilter builds the FilterCN used for the
// tls-rancher-internal (:444) listener: the pod-IP filter (which prunes
// stale rancher pod IPs) unioned with an admin-configured Service allowlist
// (settings.TLSInternalCNAllowedServices), so IPs belonging to Services the
// admin has explicitly named -- e.g. a LoadBalancer/VIP Service that isn't a
// rancher pod IP -- are never rejected/pruned by the pod-IP filter.
func newRancherInternalCNFilter(ctx context.Context, pods corev1controllers.PodController, services corev1controllers.ServiceController, allowedServices string) func(...string) []string {
	podFilter := newRancherPodIPFilter(ctx, pods)
	names := parseAllowedServices(allowedServices)
	svcFilter := newServiceIPTracker(ctx, names, services, "rancher-service-allowlist-tls-internal-filter")
	return unionFilterCN(podFilter, svcFilter)
}
