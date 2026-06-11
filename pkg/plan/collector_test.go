package plan

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// fakeCache is an in-memory SecretClient for tests. It records every List call so we can assert
// the exact selectors a Collector evaluates against the cache.
type fakeCache struct {
	secrets []*corev1.Secret
	calls   []string
	err     error
}

func (f *fakeCache) List(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s|%s", namespace, selector.String()))
	if f.err != nil {
		return nil, f.err
	}
	var out []*corev1.Secret
	for _, s := range f.secrets {
		if s.Namespace != namespace {
			continue
		}
		if !selector.Matches(labels.Set(s.Labels)) {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// fakeCluster is a minimal ClusterRef used in tests.
type fakeCluster struct{ name string }

func (c fakeCluster) GetName() string { return c.name }

// newSecret returns a labelled corev1.Secret for use in cache fixtures.
func newSecret(name string, lbls map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
			Labels:    lbls,
			UID:       types.UID(name),
		},
	}
}

// secretNames extracts names from a slice of secrets in their current order.
func secretNames(s []*corev1.Secret) []string {
	names := make([]string, 0, len(s))
	for _, sec := range s {
		names = append(names, sec.Name)
	}
	return names
}

func TestNewCollectorAutoAppliesClusterNameFilter(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("ours-a", map[string]string{labelClusterName: "mine"}),
			newSecret("ours-b", map[string]string{labelClusterName: "mine"}),
			newSecret("theirs", map[string]string{labelClusterName: "other"}),
			newSecret("orphan", nil),
		},
	}

	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := secretNames(got)
	sort.Strings(names)
	want := []string{"ours-a", "ours-b"}
	if !equalStrings(names, want) {
		t.Errorf("got %v, want %v — the cluster name filter must be auto-applied", names, want)
	}
}

func TestNewCollectorNilClusterMatchesEverything(t *testing.T) {
	t.Parallel()

	// Documented escape hatch: passing nil disables the auto cluster-name filter and returns
	// every secret in the namespace.
	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("a", map[string]string{labelClusterName: "mine"}),
			newSecret("b", map[string]string{labelClusterName: "other"}),
		},
	}
	got, err := NewCollector(cache, nil, "fleet-default").Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected both secrets, got %d (%v)", len(got), secretNames(got))
	}
}

func TestNewCollectorReturnsErrorWithoutCache(t *testing.T) {
	t.Parallel()

	_, err := (&Collector{}).Collect()
	if err == nil {
		t.Fatal("expected error when cache is nil")
	}
	if !strings.Contains(err.Error(), "SecretCache") {
		t.Errorf("error %q should mention SecretCache", err)
	}
}

func TestWithLabelsComposesWithClusterFilter(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("etcd-1", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
			newSecret("etcd-2", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
			newSecret("worker", map[string]string{labelClusterName: "mine", labelWorker: "true"}),
			// Belongs to another cluster; must not surface even though it has the etcd label.
			newSecret("foreign-etcd", map[string]string{labelClusterName: "other", labelEtcd: "true"}),
		},
	}

	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithLabels(Label(labelEtcd, "true")).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := secretNames(got)
	sort.Strings(names)
	if !equalStrings(names, []string{"etcd-1", "etcd-2"}) {
		t.Errorf("got %v, want [etcd-1 etcd-2]", names)
	}
}

func TestWithLabelsMultipleCallsAndCompose(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("etcd-init", map[string]string{labelClusterName: "mine", labelEtcd: "true", labelInitNode: "true"}),
			newSecret("etcd-only", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
		},
	}

	// Two WithLabels calls must AND together — only "etcd-init" carries both labels.
	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithLabels(Label(labelEtcd, "true")).
		WithLabels(Label(labelInitNode, "true")).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "etcd-init" {
		t.Errorf("expected only 'etcd-init', got %v", secretNames(got))
	}
}

func TestWithFilterRunsAfterFetch(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("linux-etcd", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
			newSecret("windows-etcd", map[string]string{labelClusterName: "mine", labelEtcd: "true", labelOS: osWindows}),
		},
	}

	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithLabels(Label(labelEtcd, "true")).
		WithFilter(func(s *corev1.Secret) bool { return s.Labels[labelOS] != osWindows }).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "linux-etcd" {
		t.Errorf("expected only 'linux-etcd', got %v", secretNames(got))
	}
}

func TestWithFilterMultipleAndCompose(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("a", map[string]string{labelClusterName: "mine"}),
			newSecret("ab", map[string]string{labelClusterName: "mine"}),
			newSecret("abc", map[string]string{labelClusterName: "mine"}),
		},
	}
	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithFilter(func(s *corev1.Secret) bool { return strings.HasPrefix(s.Name, "a") }).
		WithFilter(func(s *corev1.Secret) bool { return len(s.Name) >= 2 }).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := secretNames(got)
	sort.Strings(names)
	if !equalStrings(names, []string{"ab", "abc"}) {
		t.Errorf("got %v, want [ab abc]", names)
	}
}

func TestWithFilterNilIsNoOp(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{newSecret("a", map[string]string{labelClusterName: "mine"})},
	}
	_, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithFilter(nil).
		Collect()
	if err != nil {
		t.Fatalf("nil filter must be silently ignored, got: %v", err)
	}
}

func TestWithSorterApplied(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("c", map[string]string{labelClusterName: "mine"}),
			newSecret("a", map[string]string{labelClusterName: "mine"}),
			newSecret("b", map[string]string{labelClusterName: "mine"}),
		},
	}
	byName := func(s []*corev1.Secret) []*corev1.Secret {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Name < s[j].Name })
		return s
	}

	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithSorter(byName).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalStrings(secretNames(got), []string{"a", "b", "c"}) {
		t.Errorf("sort not applied, got %v", secretNames(got))
	}
}

func TestWithSorterChainOrder(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("a", map[string]string{labelClusterName: "mine"}),
			newSecret("b", map[string]string{labelClusterName: "mine"}),
		},
	}
	first := func(s []*corev1.Secret) []*corev1.Secret {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Name < s[j].Name })
		return s
	}
	second := func(s []*corev1.Secret) []*corev1.Secret {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Name > s[j].Name })
		return s
	}

	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithSorter(first).
		WithSorter(second).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Last sorter wins.
	if !equalStrings(secretNames(got), []string{"b", "a"}) {
		t.Errorf("expected last sorter to win, got %v", secretNames(got))
	}
}

func TestCacheErrorBubbles(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{err: errors.New("cache exploded")}
	_, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").Collect()
	if err == nil || !strings.Contains(err.Error(), "cache exploded") {
		t.Errorf("expected cache error to bubble, got: %v", err)
	}
}

func TestOrSelectorDeduplicatedByUID(t *testing.T) {
	t.Parallel()

	// Both the etcd and controlplane branches match `init` — it must appear in the result
	// exactly once.
	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("init", map[string]string{labelClusterName: "mine", labelEtcd: "true", labelControl: "true"}),
			newSecret("etcd-only", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
			newSecret("cp-only", map[string]string{labelClusterName: "mine", labelControl: "true"}),
		},
	}

	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithLabels(Or(Label(labelEtcd, "true"), Label(labelControl, "true"))).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := secretNames(got)
	sort.Strings(names)
	want := []string{"cp-only", "etcd-only", "init"}
	if !equalStrings(names, want) {
		t.Errorf("got %v, want %v (init must be deduplicated)", names, want)
	}
}

func TestValidatorRunsAfterFilteringAndSorting(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{
			newSecret("etcd-1", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
			newSecret("etcd-2", map[string]string{labelClusterName: "mine", labelEtcd: "true"}),
		},
	}

	var seen []string
	got, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithLabels(Label(labelEtcd, "true")).
		WithValidator(func(s []*corev1.Secret) error {
			seen = secretNames(s)
			return nil
		}).
		Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(seen) != 2 || len(got) != 2 {
		t.Fatalf("validator must see the final result set, seen=%v got=%v", seen, secretNames(got))
	}
}

func TestValidatorErrorAbortsCollect(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{newSecret("a", map[string]string{labelClusterName: "mine"})},
	}
	sentinel := errors.New("nope")
	_, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithValidator(func([]*corev1.Secret) error { return sentinel }).
		Collect()
	if !errors.Is(err, sentinel) {
		t.Errorf("validator error must bubble unchanged; got %v", err)
	}
}

func TestValidatorFirstFailureWins(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{newSecret("a", map[string]string{labelClusterName: "mine"})},
	}
	first := errors.New("first")
	called := false
	_, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithValidator(func([]*corev1.Secret) error { return first }).
		WithValidator(func([]*corev1.Secret) error { called = true; return errors.New("second") }).
		Collect()
	if !errors.Is(err, first) {
		t.Errorf("expected first validator error, got %v", err)
	}
	if called {
		t.Error("second validator should not run after the first fails")
	}
}

func TestWithValidatorNilIgnored(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{
		secrets: []*corev1.Secret{newSecret("a", map[string]string{labelClusterName: "mine"})},
	}
	_, err := NewCollector(cache, fakeCluster{name: "mine"}, "fleet-default").
		WithValidator(nil, nil).
		Collect()
	if err != nil {
		t.Errorf("nil validators must be silently ignored, got %v", err)
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()

	a := newSecret("a", map[string]string{labelEtcd: "true"})
	b := newSecret("b", map[string]string{labelEtcd: "true"})
	c := newSecret("c", nil)

	cases := []struct {
		name    string
		set     []*corev1.Secret
		key     string
		n       int
		wantErr bool
	}{
		{"empty label = count", []*corev1.Secret{a, b, c}, "", 2, false},
		{"empty label, too few", []*corev1.Secret{a}, "", 2, true},
		{"by label, exactly meets", []*corev1.Secret{a, b}, labelEtcd, 2, false},
		{"by label, exceeds", []*corev1.Secret{a, b, c}, labelEtcd, 1, false},
		{"by label, too few", []*corev1.Secret{a, c}, labelEtcd, 2, true},
		{"empty set, n=0", nil, "", 0, false},
		{"empty set, n=1", nil, labelEtcd, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := AtLeast(tc.n, tc.key)(tc.set)
			if (err != nil) != tc.wantErr {
				t.Errorf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestAtMost(t *testing.T) {
	t.Parallel()

	a := newSecret("a", map[string]string{labelEtcd: "true"})
	b := newSecret("b", map[string]string{labelEtcd: "true"})

	cases := []struct {
		name    string
		set     []*corev1.Secret
		key     string
		n       int
		wantErr bool
	}{
		{"at limit", []*corev1.Secret{a, b}, labelEtcd, 2, false},
		{"under limit", []*corev1.Secret{a}, labelEtcd, 2, false},
		{"over limit", []*corev1.Secret{a, b}, labelEtcd, 1, true},
		{"empty label, at limit", []*corev1.Secret{a}, "", 1, false},
		{"empty label, over", []*corev1.Secret{a, b}, "", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := AtMost(tc.n, tc.key)(tc.set)
			if (err != nil) != tc.wantErr {
				t.Errorf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestExactly(t *testing.T) {
	t.Parallel()

	a := newSecret("a", map[string]string{labelInitNode: "true"})
	b := newSecret("b", map[string]string{labelInitNode: "true"})

	if err := Exactly(1, labelInitNode)([]*corev1.Secret{a}); err != nil {
		t.Errorf("exact match should pass, got %v", err)
	}
	if err := Exactly(1, labelInitNode)([]*corev1.Secret{a, b}); err == nil {
		t.Error("too many should fail")
	}
	if err := Exactly(1, labelInitNode)(nil); err == nil {
		t.Error("too few should fail")
	}
}

func TestDefaultSorterRoleOrdering(t *testing.T) {
	t.Parallel()

	mk := func(name string, role ...string) *corev1.Secret {
		lbls := map[string]string{}
		for _, r := range role {
			lbls[r] = "true"
		}
		return newSecret(name, lbls)
	}
	in := []*corev1.Secret{
		mk("z-worker", labelWorker),
		mk("etcd-cp-worker", labelEtcd, labelControl, labelWorker),
		mk("init-etcd", labelEtcd, labelInitNode),
		mk("etcd", labelEtcd),
		mk("etcd-cp", labelEtcd, labelControl),
		mk("cp", labelControl),
	}

	got := DefaultSorter()(in)
	want := []string{"init-etcd", "etcd", "etcd-cp", "etcd-cp-worker", "cp", "z-worker"}
	if !equalStrings(secretNames(got), want) {
		t.Errorf("got %v, want %v", secretNames(got), want)
	}
}

func TestDefaultSorterTiebreakByName(t *testing.T) {
	t.Parallel()

	in := []*corev1.Secret{
		newSecret("b", map[string]string{labelEtcd: "true"}),
		newSecret("a", map[string]string{labelEtcd: "true"}),
	}
	got := DefaultSorter()(in)
	if !equalStrings(secretNames(got), []string{"a", "b"}) {
		t.Errorf("expected lex tiebreak, got %v", secretNames(got))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
