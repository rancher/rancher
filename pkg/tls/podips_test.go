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
)

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

	// Snapshot was never populated (still pre-sync / nil), so IP CNs still
	// pass through -- confirms List() was correctly skipped.
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

	// Snapshot remains nil (pre-sync) since the list failed, so IP CNs
	// still pass through rather than being incorrectly pruned.
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
