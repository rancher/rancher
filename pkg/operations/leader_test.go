package operations

import (
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func roleSecret(name string, roles ...string) *corev1.Secret {
	labels := map[string]string{}
	for _, r := range roles {
		labels[r] = "true"
	}
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func candidate(secret *corev1.Secret, eligible, init bool) LeaderCandidate {
	return LeaderCandidate{Secret: secret, Eligible: eligible, Init: init}
}

func TestLeaderRoleString(t *testing.T) {
	t.Parallel()

	cases := map[LeaderRole]string{
		LeaderRole(0):                              "none",
		LeaderRoleEtcd:                             "etcd",
		LeaderRoleControlPlane:                     "controlplane",
		LeaderRoleEtcd | LeaderRoleControlPlane:    "etcd+controlplane",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("LeaderRole(%d).String() = %q, want %q", r, got, want)
		}
	}
}

func TestSecretRoleSet(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		secret *corev1.Secret
		want   LeaderRole
	}{
		"nil secret":      {nil, 0},
		"unlabeled":       {&corev1.Secret{}, 0},
		"etcd only":       {roleSecret("a", capr.EtcdRoleLabel), LeaderRoleEtcd},
		"controlplane":    {roleSecret("a", capr.ControlPlaneRoleLabel), LeaderRoleControlPlane},
		"etcd + cp":       {roleSecret("a", capr.EtcdRoleLabel, capr.ControlPlaneRoleLabel), LeaderRoleEtcd | LeaderRoleControlPlane},
		"worker (no bit)": {roleSecret("a", capr.WorkerRoleLabel), 0},
	}
	for name, tc := range cases {
		if got := secretRoleSet(tc.secret); got != tc.want {
			t.Errorf("%s: secretRoleSet = %v, want %v", name, got, tc.want)
		}
	}
}

func TestElectLeaderPrefersInit(t *testing.T) {
	t.Parallel()

	etcdOnly := roleSecret("z-etcd", capr.EtcdRoleLabel)
	initSecret := roleSecret("a-init", capr.EtcdRoleLabel)

	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(etcdOnly, true, false),
		candidate(initSecret, true, true),
	})
	if got != initSecret {
		// Init candidate wins even though etcdOnly would beat it lexicographically without the
		// init flag (z-etcd vs a-init — irrelevant; tier 0 beats tier 1 regardless of name).
		t.Errorf("expected init candidate, got %v", got)
	}
}

func TestElectLeaderPrefersExactRoleMatch(t *testing.T) {
	t.Parallel()

	etcdOnly := roleSecret("z-etcd", capr.EtcdRoleLabel)
	etcdCP := roleSecret("a-etcd-cp", capr.EtcdRoleLabel, capr.ControlPlaneRoleLabel)

	// For LeaderRoleEtcd, etcd-only is the exact match — tier 1 beats tier 2 (etcd+cp).
	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(etcdCP, true, false),
		candidate(etcdOnly, true, false),
	})
	if got != etcdOnly {
		t.Errorf("expected etcd-only, got %v", got)
	}
}

func TestElectLeaderTiebreakLex(t *testing.T) {
	t.Parallel()

	a := roleSecret("a", capr.EtcdRoleLabel)
	b := roleSecret("b", capr.EtcdRoleLabel)
	c := roleSecret("c", capr.EtcdRoleLabel)

	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(c, true, false),
		candidate(a, true, false),
		candidate(b, true, false),
	})
	if got != a {
		t.Errorf("expected lex-smallest 'a', got %v", got)
	}
}

func TestElectLeaderSkipsIneligible(t *testing.T) {
	t.Parallel()

	deleting := roleSecret("a", capr.EtcdRoleLabel)
	ok := roleSecret("b", capr.EtcdRoleLabel)

	// The lex-smallest is ineligible — election must skip it for the next eligible.
	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(deleting, false, false),
		candidate(ok, true, false),
	})
	if got != ok {
		t.Errorf("expected 'b' (lex-smallest eligible), got %v", got)
	}
}

func TestElectLeaderReturnsNilWhenNoneEligible(t *testing.T) {
	t.Parallel()

	a := roleSecret("a", capr.EtcdRoleLabel)
	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(a, false, false),
	})
	if got != nil {
		t.Errorf("expected nil when no candidate is eligible, got %v", got)
	}
}

func TestElectLeaderReturnsNilWhenRoleMissing(t *testing.T) {
	t.Parallel()

	cp := roleSecret("a", capr.ControlPlaneRoleLabel)
	// Asking for etcd from a pool of cp-only candidates yields nothing.
	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(cp, true, false),
	})
	if got != nil {
		t.Errorf("expected nil when no candidate holds the requested role, got %v", got)
	}
}

func TestElectLeaderConglomerateRole(t *testing.T) {
	t.Parallel()

	// Asking for etcd+cp must require both roles on a single candidate; etcd-only does not qualify.
	etcdOnly := roleSecret("a-etcd", capr.EtcdRoleLabel)
	etcdCP := roleSecret("b-etcd-cp", capr.EtcdRoleLabel, capr.ControlPlaneRoleLabel)

	got := electLeader(LeaderRoleEtcd|LeaderRoleControlPlane, []LeaderCandidate{
		candidate(etcdOnly, true, false),
		candidate(etcdCP, true, false),
	})
	if got != etcdCP {
		t.Errorf("expected etcd+cp candidate, got %v", got)
	}
}

func TestElectLeaderConglomerateRoleInitWinsOnlyIfHoldsAllRoles(t *testing.T) {
	t.Parallel()

	// An init etcd-only candidate must NOT win when cp is also required — it doesn't hold the
	// full requested role set.
	initEtcdOnly := roleSecret("a-init-etcd", capr.EtcdRoleLabel)
	etcdCP := roleSecret("z-etcd-cp", capr.EtcdRoleLabel, capr.ControlPlaneRoleLabel)

	got := electLeader(LeaderRoleEtcd|LeaderRoleControlPlane, []LeaderCandidate{
		candidate(initEtcdOnly, true, true),
		candidate(etcdCP, true, false),
	})
	if got != etcdCP {
		t.Errorf("init etcd-only must be filtered out when cp is required; got %v", got)
	}
}

func TestElectLeaderStableUnderIrrelevantChanges(t *testing.T) {
	t.Parallel()

	// "Candidates equal in suitability should not yield" — re-running with the same lex-sorted
	// candidates must return the same secret. Add an extra worker-only candidate (not in the
	// requested role set) and verify the existing winner is unchanged.
	etcdOnly := roleSecret("a-etcd", capr.EtcdRoleLabel)
	workerOnly := roleSecret("a-worker", capr.WorkerRoleLabel)

	first := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(etcdOnly, true, false),
	})
	second := electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(workerOnly, true, false), // irrelevant — doesn't hold etcd
		candidate(etcdOnly, true, false),
	})
	if first != second || first != etcdOnly {
		t.Errorf("election should be stable across irrelevant candidates; first=%v second=%v", first, second)
	}
}

func TestElectLeaderPromotesUnderRoleAddition(t *testing.T) {
	t.Parallel()

	// "An etcd-only leader that has the controlplane role added to it should undergo election as
	// normal." Re-electing with the same candidate now holding etcd+cp must still pick it when
	// it's the only etcd candidate.
	before := roleSecret("a", capr.EtcdRoleLabel)
	after := roleSecret("a", capr.EtcdRoleLabel, capr.ControlPlaneRoleLabel)

	got := electLeader(LeaderRoleEtcd, []LeaderCandidate{candidate(after, true, false)})
	if got != after {
		t.Errorf("etcd+cp candidate must remain etcd leader when it is the sole etcd-bearing candidate; got %v", got)
	}

	// And: re-elect against the *both* — etcd-only beats etcd+cp at tier 1 by virtue of exact role
	// match. The just-promoted etcd+cp yields to a fresh etcd-only candidate, as the user said
	// "leader election should defer to a more suitable candidate".
	freshEtcdOnly := roleSecret("z-fresh", capr.EtcdRoleLabel)
	got = electLeader(LeaderRoleEtcd, []LeaderCandidate{
		candidate(before, true, false),       // baseline existing (kept for context, not used)
		candidate(after, true, false),        // promoted to etcd+cp
		candidate(freshEtcdOnly, true, false), // new etcd-only candidate
	})
	if got != before && got != freshEtcdOnly {
		t.Errorf("expected a tier-1 (etcd-only) candidate, got %v", got)
	}
	// Lex tiebreak: "a" < "z-fresh", so "before" wins (it's also etcd-only by label).
	if got != before {
		t.Errorf("expected lex-smallest etcd-only 'a', got %v", got)
	}
}

func TestIsImportedInitNodeK3s(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []byte
		want bool
	}{
		"server with --cluster-init":    {[]byte(`["server","--cluster-init"]`), true},
		"server with --cluster-init=true": {[]byte(`["server","--cluster-init=true"]`), true},
		"server without --cluster-init": {[]byte(`["server","--node-name=foo"]`), false},
		"agent":                          {[]byte(`["agent","--server","https://x"]`), false},
		"empty":                          {[]byte(``), false},
		"invalid json":                   {[]byte(`not-json`), false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			node := &mgmtv3.Node{
				Status: mgmtv3.NodeStatus{
					NodeAnnotations: map[string]string{"k3s.io/node-args": string(tc.args)},
				},
			}
			if got := isImportedInitNode(node, capr.RuntimeK3S); got != tc.want {
				t.Errorf("isImportedInitNode k3s args=%s = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestIsImportedInitNodeRKE2(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []byte
		want bool
	}{
		"server without --server":  {[]byte(`["server","--config","/etc/rancher/rke2/config.yaml"]`), true},
		"server with --server":     {[]byte(`["server","--server","https://x:9345"]`), false},
		"server with --server=...": {[]byte(`["server","--server=https://x:9345"]`), false},
		"agent":                    {[]byte(`["agent","--server","https://x:9345"]`), false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			node := &mgmtv3.Node{
				Status: mgmtv3.NodeStatus{
					NodeAnnotations: map[string]string{"rke2.io/node-args": string(tc.args)},
				},
			}
			if got := isImportedInitNode(node, capr.RuntimeRKE2); got != tc.want {
				t.Errorf("isImportedInitNode rke2 args=%s = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestIsImportedInitNodeMissingAnnotation(t *testing.T) {
	t.Parallel()

	// Nodes whose syncer hasn't populated NodeAnnotations yet must not be treated as init.
	if isImportedInitNode(&mgmtv3.Node{}, capr.RuntimeRKE2) {
		t.Error("missing annotations must not yield init=true")
	}
}
