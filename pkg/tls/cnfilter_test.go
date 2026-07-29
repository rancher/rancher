package tls

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestParseAllowedServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		expected []serviceRef
	}{
		{
			name:     "empty value",
			value:    "",
			expected: nil,
		},
		{
			name:     "single bare name defaults to cattle-system",
			value:    "my-vip-service",
			expected: []serviceRef{{namespace: namespace.System, name: "my-vip-service"}},
		},
		{
			name:     "namespace/name pair overrides default",
			value:    "other-ns/other-service",
			expected: []serviceRef{{namespace: "other-ns", name: "other-service"}},
		},
		{
			name:  "multiple entries, mixed forms, whitespace tolerated, blanks dropped",
			value: " my-vip-service , other-ns/other-service ,,",
			expected: []serviceRef{
				{namespace: namespace.System, name: "my-vip-service"},
				{namespace: "other-ns", name: "other-service"},
			},
		},
		{
			name:     "malformed namespace/name entry (empty name) is ignored",
			value:    "other-ns/",
			expected: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseAllowedServices(tt.value)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseAllowedServices(%q) = %#v, want %#v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestUnionFilterCN(t *testing.T) {
	t.Parallel()

	// primary keeps only "10.0.0.1"; allowlist keeps only "192.168.10.131".
	primary := func(cns ...string) []string {
		var out []string
		for _, cn := range cns {
			if cn == "10.0.0.1" {
				out = append(out, cn)
			}
		}
		return out
	}
	allowlist := func(cns ...string) []string {
		var out []string
		for _, cn := range cns {
			if cn == "192.168.10.131" {
				out = append(out, cn)
			}
		}
		return out
	}

	union := unionFilterCN(primary, allowlist)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "primary accepts everything — allowlist never consulted",
			input:    []string{"10.0.0.1"},
			expected: []string{"10.0.0.1"},
		},
		{
			name:     "primary rejects, allowlist accepts — union keeps it",
			input:    []string{"192.168.10.131"},
			expected: []string{"192.168.10.131"},
		},
		{
			name:     "primary and allowlist both reject — dropped",
			input:    []string{"10.0.0.2"},
			expected: nil,
		},
		{
			name:     "mixed: one from each, one from neither",
			input:    []string{"10.0.0.1", "192.168.10.131", "10.0.0.2"},
			expected: []string{"10.0.0.1", "192.168.10.131"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := union(tt.input...)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("union(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// makeService builds a Service with the given ClusterIP, ExternalIPs, and
// LoadBalancer ingress IPs for use in serviceIPTracker tests.
func makeService(ns, name, clusterIP string, externalIPs []string, lbIPs ...string) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: corev1.ServiceSpec{
			ClusterIP:   clusterIP,
			ExternalIPs: externalIPs,
		},
	}
	for _, ip := range lbIPs {
		svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{IP: ip})
	}
	return svc
}

func TestServiceIPTracker(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	services := fake.NewMockControllerInterface[*corev1.Service, *corev1.ServiceList](ctrl)
	services.EXPECT().
		OnChange(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	svc := makeService(namespace.System, "harvester-vip", "10.43.1.1", []string{"192.168.10.131"}, "192.168.10.132")
	services.EXPECT().
		Get(namespace.System, "harvester-vip", gomock.Any()).
		Return(svc, nil).
		AnyTimes()

	otherNsSvc := makeService("other-ns", "other-vip", "10.43.2.2", nil, "172.16.0.5")
	services.EXPECT().
		Get("other-ns", "other-vip", gomock.Any()).
		Return(otherNsSvc, nil).
		AnyTimes()

	refs := []serviceRef{
		{namespace: namespace.System, name: "harvester-vip"},
		{namespace: "other-ns", name: "other-vip"},
	}
	filter := newServiceIPTracker(context.Background(), refs, services, "test-service-allowlist")

	got := filter("10.43.1.1", "192.168.10.131", "192.168.10.132", "10.43.2.2", "172.16.0.5", "10.0.0.9")
	expected := []string{"10.43.1.1", "192.168.10.131", "192.168.10.132", "10.43.2.2", "172.16.0.5"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("filter(...) = %v, want %v", got, expected)
	}
}

func TestServiceIPTracker_OnChange_ScopesByNamespaceAndName(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	services := fake.NewMockControllerInterface[*corev1.Service, *corev1.ServiceList](ctrl)

	svc := makeService(namespace.System, "harvester-vip", "10.43.1.1", nil)
	services.EXPECT().
		Get(namespace.System, "harvester-vip", gomock.Any()).
		Return(svc, nil).
		Times(1)

	refs := []serviceRef{{namespace: namespace.System, name: "harvester-vip"}}
	tracker := &serviceIPTracker{refs: refs, services: services}
	empty := map[string]struct{}{}
	tracker.ips.Store(&empty)

	// A Service with the same name but a *different* namespace must not be
	// treated as watched -- it should not trigger a rebuild.
	sameNameOtherNs := makeService("other-ns", "harvester-vip", "10.99.9.9", nil)
	if _, err := tracker.onChange("other-ns/harvester-vip", sameNameOtherNs); err != nil {
		t.Fatalf("onChange returned error: %v", err)
	}
	if got := tracker.allowCN("10.99.9.9"); len(got) != 0 {
		t.Errorf("expected same-name-different-namespace Service to be ignored, got %v", got)
	}

	// The actual watched Service (same namespace and name) must trigger a
	// rebuild and have its ClusterIP allowed.
	if _, err := tracker.onChange(namespace.System+"/harvester-vip", svc); err != nil {
		t.Fatalf("onChange returned error: %v", err)
	}
	if got := tracker.allowCN("10.43.1.1"); !reflect.DeepEqual(got, []string{"10.43.1.1"}) {
		t.Errorf("expected watched Service's ClusterIP to be allowed, got %v", got)
	}
}

func TestServiceIPTracker_NoRefsIsNoop(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	services := fake.NewMockControllerInterface[*corev1.Service, *corev1.ServiceList](ctrl)
	// No OnChange/Get calls expected at all when there are no refs.

	filter := newServiceIPTracker(context.Background(), nil, services, "test-service-allowlist-empty")
	got := filter("192.168.10.131")
	if len(got) != 0 {
		t.Errorf("expected no CNs allowed with empty refs, got %v", got)
	}
}

func TestServiceIPTracker_MissingServiceIsIgnored(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	services := fake.NewMockControllerInterface[*corev1.Service, *corev1.ServiceList](ctrl)
	services.EXPECT().
		OnChange(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	services.EXPECT().
		Get(namespace.System, "does-not-exist", gomock.Any()).
		Return(nil, apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, "does-not-exist")).
		AnyTimes()

	refs := []serviceRef{{namespace: namespace.System, name: "does-not-exist"}}
	filter := newServiceIPTracker(context.Background(), refs, services, "test-service-allowlist-missing")

	got := filter("192.168.10.131")
	if len(got) != 0 {
		t.Errorf("expected no CNs allowed when Service lookup fails, got %v", got)
	}
}

// makePod builds a Pod with the given namespace, name, IP, and (optional)
// deletion timestamp for use in podIPTracker tests.
func makePod(ns, name, ip string, deleted bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Status:     corev1.PodStatus{PodIP: ip},
	}
	if deleted {
		now := metav1.Now()
		pod.DeletionTimestamp = &now
	}
	return pod
}

func TestPodIPTracker_FilterExistingCN_PreSync(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pods := fake.NewMockControllerInterface[*corev1.Pod, *corev1.PodList](ctrl)
	pods.EXPECT().
		OnChange(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	// List must never be called before the first pod event fires.

	filter := newPodIPTracker(context.Background(), namespace.System, "app=rancher", pods, "test-podip-filter-presync")

	// Pre-sync: keep IP CNs since we don't yet know the live pod set and
	// must not prematurely prune a legitimate one. Hostnames are always
	// rejected, pre-sync or not.
	got := filter("10.42.0.1", "rancher.cattle-system", "192.168.10.131")
	expected := []string{"10.42.0.1", "192.168.10.131"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("pre-sync filter(...) = %v, want %v", got, expected)
	}
}

func TestPodIPTracker_OnChange_AndFilter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pods := fake.NewMockControllerInterface[*corev1.Pod, *corev1.PodList](ctrl)

	list := &corev1.PodList{
		Items: []corev1.Pod{
			*makePod(namespace.System, "rancher-1", "10.42.0.1", false),
			*makePod(namespace.System, "rancher-2", "10.42.0.2", false),
			*makePod(namespace.System, "rancher-3", "10.42.0.3", true), // being deleted, excluded
			*makePod(namespace.System, "rancher-4", "", false),         // no IP yet, excluded
		},
	}
	pods.EXPECT().
		List(namespace.System, gomock.Any()).
		Return(list, nil).
		AnyTimes()

	// Capture the real OnChange handler newPodIPTracker registers, so the
	// test exercises the actual public entrypoint end-to-end rather than
	// calling podIPTracker's unexported methods directly.
	var handler func(string, *corev1.Pod) (*corev1.Pod, error)
	pods.EXPECT().
		OnChange(gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, _ string, h func(string, *corev1.Pod) (*corev1.Pod, error)) {
			handler = h
		}).
		Times(1)

	filter := newPodIPTracker(context.Background(), namespace.System, "app=rancher", pods, "test-podip-filter")

	// Simulate the informer delivering the first pod event, which triggers
	// a List() and populates the snapshot.
	if _, err := handler("rancher-1", &list.Items[0]); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "live pod IPs kept",
			input:    []string{"10.42.0.1", "10.42.0.2"},
			expected: []string{"10.42.0.1", "10.42.0.2"},
		},
		{
			name:     "stale/unknown IP pruned",
			input:    []string{"10.42.0.99"},
			expected: []string{},
		},
		{
			name:     "deleted pod's IP pruned even though it was in the list response",
			input:    []string{"10.42.0.3"},
			expected: []string{},
		},
		{
			name:     "hostnames always rejected regardless of pod set",
			input:    []string{"rancher.cattle-system", "some.other.host"},
			expected: []string{},
		},
		{
			name:     "mixed live IP, stale IP, and hostname",
			input:    []string{"10.42.0.1", "10.42.0.99", "rancher.cattle-system"},
			expected: []string{"10.42.0.1"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := filter(tt.input...)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("filter(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPodIPTracker_OnChange_SkipsOtherNamespaces(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pods := fake.NewMockControllerInterface[*corev1.Pod, *corev1.PodList](ctrl)
	// List must never be called for an event on a pod outside our namespace.

	tracker := &podIPTracker{namespace: namespace.System, labelSelector: "app=rancher", pods: pods}
	otherNsPod := makePod("some-other-namespace", "unrelated", "10.99.0.1", false)
	if _, err := tracker.onChange("unrelated", otherNsPod); err != nil {
		t.Fatalf("onChange returned error: %v", err)
	}

	// Snapshot was never populated (still pre-sync / nil), so everything
	// passes through -- confirms List() was correctly skipped.
	got := tracker.filterExistingCN("10.99.0.1")
	if !reflect.DeepEqual(got, []string{"10.99.0.1"}) {
		t.Errorf("expected pass-through pre-sync behavior, got %v", got)
	}
}

func TestPodIPTracker_OnChange_DeleteEventRelists(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pods := fake.NewMockControllerInterface[*corev1.Pod, *corev1.PodList](ctrl)
	list := &corev1.PodList{
		Items: []corev1.Pod{
			*makePod(namespace.System, "rancher-1", "10.42.0.1", false),
		},
	}
	pods.EXPECT().
		List(namespace.System, gomock.Any()).
		Return(list, nil).
		Times(1)

	tracker := &podIPTracker{namespace: namespace.System, labelSelector: "app=rancher", pods: pods}

	// Delete events arrive with a nil pod; onChange must still re-list to
	// pick up the pod's removal from the live set.
	if _, err := tracker.onChange("rancher-2", nil); err != nil {
		t.Fatalf("onChange returned error: %v", err)
	}

	got := tracker.filterExistingCN("10.42.0.1", "10.42.0.2")
	if !reflect.DeepEqual(got, []string{"10.42.0.1"}) {
		t.Errorf("filterExistingCN(...) = %v, want [10.42.0.1]", got)
	}
}

func TestPodIPTracker_OnChange_ListError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pods := fake.NewMockControllerInterface[*corev1.Pod, *corev1.PodList](ctrl)
	pods.EXPECT().
		List(namespace.System, gomock.Any()).
		Return(nil, apierrors.NewInternalError(errors.New("boom"))).
		Times(1)

	tracker := &podIPTracker{namespace: namespace.System, labelSelector: "app=rancher", pods: pods}
	pod := makePod(namespace.System, "rancher-1", "10.42.0.1", false)
	if _, err := tracker.onChange("rancher-1", pod); err == nil {
		t.Fatal("expected error from onChange when List fails")
	}

	// Snapshot remains nil (pre-sync) since the list failed, so everything
	// still passes through rather than being incorrectly pruned.
	got := tracker.filterExistingCN("10.42.0.1")
	if !reflect.DeepEqual(got, []string{"10.42.0.1"}) {
		t.Errorf("expected pass-through after failed List, got %v", got)
	}
}

// TestPodIPTracker_RejectsArbitraryHostnameInjection is a regression test
// for the CN-filtering behavior: a client that can reach the
// tls-rancher-internal listener and control TLS SNI or the HTTP Host header
// must never be able to get an arbitrary hostname added to the cert, no
// matter how many distinct hostnames are attempted or what the live pod-IP
// snapshot contains. Only the static default SANs (already applied upstream
// by dynamiclistener's allowDefaultSANs, before this filter is ever
// consulted) and live pod IPs may appear on the cert.
func TestPodIPTracker_RejectsArbitraryHostnameInjection(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pods := fake.NewMockControllerInterface[*corev1.Pod, *corev1.PodList](ctrl)
	list := &corev1.PodList{
		Items: []corev1.Pod{
			*makePod(namespace.System, "rancher-1", "10.42.0.13", false),
		},
	}
	pods.EXPECT().
		List(namespace.System, gomock.Any()).
		Return(list, nil).
		AnyTimes()
	pods.EXPECT().
		OnChange(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	// Populate the snapshot the way the real onChange handler would.
	tracker := &podIPTracker{namespace: namespace.System, labelSelector: "app=rancher", pods: pods}
	if _, err := tracker.onChange("rancher-1", &list.Items[0]); err != nil {
		t.Fatalf("onChange returned error: %v", err)
	}
	filter := tracker.filterExistingCN

	// Simulate a client sending many distinct fake SNI hostnames, as would
	// happen via repeated TLS ClientHello / HTTP Host header requests
	// against the pod's live IP.
	extraHostnames := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		extraHostnames = append(extraHostnames, fmt.Sprintf("san-test-extra-%d.example", i))
	}

	got := filter(extraHostnames...)
	if len(got) != 0 {
		t.Errorf("expected all %d extra hostnames to be rejected, but %d got through: %v", len(extraHostnames), len(got), got)
	}

	// A legitimate live pod IP must still be kept alongside the rejected
	// hostnames in the same call.
	mixed := append([]string{"10.42.0.13"}, extraHostnames...)
	got = filter(mixed...)
	if !reflect.DeepEqual(got, []string{"10.42.0.13"}) {
		t.Errorf("filter(pod IP + extra hostnames) = %v, want only the live pod IP kept", got)
	}
}
