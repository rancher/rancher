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

// serviceRef identifies a Service by namespace/name.
type serviceRef struct {
	namespace string
	name      string
}

// parseAllowedServices parses the comma-separated
// settings.TLSInternalCNAllowedServices value into serviceRefs. Each entry
// is either a bare Service name (assumed to live in cattle-system, since
// that's where Rancher's own pods run) or a "namespace/name" pair for
// Services that live elsewhere (e.g. a VIP-granting Service managed by
// separate infrastructure outside cattle-system). Blank/whitespace-only
// entries are ignored.
func parseAllowedServices(value string) []serviceRef {
	var refs []serviceRef
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		ns := namespace.System
		name := entry
		if idx := strings.Index(entry, "/"); idx >= 0 {
			ns = entry[:idx]
			name = entry[idx+1:]
		}
		if ns == "" || name == "" {
			continue
		}
		refs = append(refs, serviceRef{namespace: ns, name: name})
	}
	return refs
}

// serviceIPTracker watches a fixed, admin-configured list of Services
// (cattle-system by default, or any namespace explicitly named via
// "namespace/name") and maintains a snapshot of their current IPs
// (ClusterIP, ExternalIPs, and any LoadBalancer ingress IPs). Unlike
// podIPTracker, this is an *allowlist*: any CN matching an IP in the
// snapshot is always kept, regardless of what other filters would
// otherwise decide. This lets an admin explicitly trust a Service's IP
// (e.g. a LoadBalancer/VIP fronting Rancher, possibly managed by
// infrastructure outside cattle-system) that the pod-IP filter has no way
// to recognize as legitimate, without disabling pruning entirely (which is
// what the listener.cattle.io/static workaround does).
type serviceIPTracker struct {
	// ips holds *map[string]struct{}, always non-nil after construction.
	ips atomic.Value

	refs     []serviceRef
	services corev1controllers.ServiceController
}

// newServiceIPTracker registers an OnChange handler on Services that
// rebuilds the allowed-IP snapshot whenever a watched Service changes. If
// refs is empty, the returned filter is a pass-through no-op (nothing extra
// is allowed).
func newServiceIPTracker(ctx context.Context, refs []serviceRef, services corev1controllers.ServiceController, handlerName string) func(...string) []string {
	t := &serviceIPTracker{
		refs:     refs,
		services: services,
	}
	empty := map[string]struct{}{}
	t.ips.Store(&empty)

	if len(refs) == 0 {
		return t.allowCN
	}

	// OnChange watches Services across all namespaces; watched() below
	// scopes matching down to the specific namespace/name pairs we care
	// about, since refs may span multiple namespaces.
	services.OnChange(ctx, handlerName, t.onChange)
	// Populate synchronously with whatever is available right away, so we
	// don't need to wait for the informer to deliver its first events
	// before the allowlist is usable.
	t.rebuild()
	return t.allowCN
}

func (t *serviceIPTracker) watched(ns, name string) bool {
	for _, ref := range t.refs {
		if ref.namespace == ns && ref.name == name {
			return true
		}
	}
	return false
}

func (t *serviceIPTracker) onChange(_ string, svc *corev1.Service) (*corev1.Service, error) {
	// On delete events svc is nil; either way, only react to Services we
	// actually care about, then rebuild the whole snapshot from the live
	// set (cheap: at most a handful of watched Services).
	if svc != nil && !t.watched(svc.Namespace, svc.Name) {
		return svc, nil
	}
	t.rebuild()
	return svc, nil
}

func (t *serviceIPTracker) rebuild() {
	ips := make(map[string]struct{})
	for _, ref := range t.refs {
		svc, err := t.services.Get(ref.namespace, ref.name, metav1.GetOptions{})
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
//
// primary's rejected set is determined by set membership against its own
// output, not by comparing lengths -- primary may transform its input
// (e.g. collapse it down to a single hostname) rather than merely filtering
// a subset of it, so an equal-length result does not imply nothing was
// rejected.
func unionFilterCN(primary, allowlist func(...string) []string) func(...string) []string {
	return func(cns ...string) []string {
		kept := primary(cns...)
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
// FilterExistingCN closure. handlerName distinguishes each listener's
// tracker instance (e.g. for metrics/logging) since this is called once per
// listener (:443 and :444).
func newRancherPodIPFilter(ctx context.Context, pods corev1controllers.PodController, handlerName string) func(...string) []string {
	return newPodIPTracker(ctx, namespace.System, "app=rancher", pods, handlerName)
}

// newRancherInternalCNFilter builds the FilterCN used for the
// tls-rancher-internal (:444) listener: the pod-IP filter (which prunes
// stale rancher pod IPs) unioned with an admin-configured Service allowlist
// (settings.TLSInternalCNAllowedServices), so IPs belonging to Services the
// admin has explicitly named -- e.g. a LoadBalancer/VIP Service that isn't a
// rancher pod IP -- are never rejected/pruned by the pod-IP filter.
func newRancherInternalCNFilter(ctx context.Context, pods corev1controllers.PodController, services corev1controllers.ServiceController, allowedServices string) func(...string) []string {
	podFilter := newRancherPodIPFilter(ctx, pods, "rancher-podip-tls-internal-filter")
	refs := parseAllowedServices(allowedServices)
	svcFilter := newServiceIPTracker(ctx, refs, services, "rancher-service-allowlist-tls-internal-filter")
	return unionFilterCN(podFilter, svcFilter)
}
