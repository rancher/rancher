package tls

import (
	"context"
	"errors"
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
		expected []string
	}{
		{
			name:     "empty value",
			value:    "",
			expected: nil,
		},
		{
			name:     "single name",
			value:    "my-vip-service",
			expected: []string{"my-vip-service"},
		},
		{
			name:     "multiple entries, whitespace tolerated, blanks dropped",
			value:    " my-vip-service , other-vip-service ,,",
			expected: []string{"my-vip-service", "other-vip-service"},
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

	refs := []string{"harvester-vip"}
	filter := newServiceIPTracker(context.Background(), refs, services, "test-service-allowlist")

	got := filter("10.43.1.1", "192.168.10.131", "192.168.10.132", "10.0.0.9")
	expected := []string{"10.43.1.1", "192.168.10.131", "192.168.10.132"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("filter(...) = %v, want %v", got, expected)
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

	refs := []string{"does-not-exist"}
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

	// Pre-sync: keep everything, including IPs, since we don't yet know the
	// live pod set and must not prematurely prune a legitimate CN.
	input := []string{"10.42.0.1", "rancher.cattle-system", "192.168.10.131"}
	got := filter(input...)
	if !reflect.DeepEqual(got, input) {
		t.Errorf("pre-sync filter(%v) = %v, want pass-through %v", input, got, input)
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
			name:     "hostnames always pass through regardless of pod set",
			input:    []string{"rancher.cattle-system", "some.other.host"},
			expected: []string{"rancher.cattle-system", "some.other.host"},
		},
		{
			name:     "mixed live IP, stale IP, and hostname",
			input:    []string{"10.42.0.1", "10.42.0.99", "rancher.cattle-system"},
			expected: []string{"10.42.0.1", "rancher.cattle-system"},
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
