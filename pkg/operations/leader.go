package operations

import (
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
)

// LeaderRole is a bitmask of the cluster roles a machine-plan secret holds. Election asks the
// adapter for a leader for one or more roles (composed via bitwise-or), and returns the most
// suitable secret whose role set is a superset of the requested mask.
//
// For example, ElectLeader(LeaderRoleEtcd|LeaderRoleControlPlane, ...) elects a leader whose
// secret carries BOTH role labels; ElectLeader(LeaderRoleEtcd, ...) accepts any etcd-bearing
// secret (etcd-only or etcd+controlplane).
type LeaderRole int

const (
	// LeaderRoleEtcd corresponds to capr.EtcdRoleLabel="true".
	LeaderRoleEtcd LeaderRole = 1 << iota
	// LeaderRoleControlPlane corresponds to capr.ControlPlaneRoleLabel="true".
	LeaderRoleControlPlane
)

// String returns "etcd", "controlplane", "etcd+controlplane", etc.
func (r LeaderRole) String() string {
	var parts []string
	if r&LeaderRoleEtcd != 0 {
		parts = append(parts, "etcd")
	}
	if r&LeaderRoleControlPlane != 0 {
		parts = append(parts, "controlplane")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "+")
}

// secretRoleSet extracts the LeaderRole bitmask from the secret's role labels. A nil or unlabeled
// secret returns the zero role set.
func secretRoleSet(secret *corev1.Secret) LeaderRole {
	var r LeaderRole
	if secret == nil || secret.Labels == nil {
		return r
	}
	if secret.Labels[capr.EtcdRoleLabel] == "true" {
		r |= LeaderRoleEtcd
	}
	if secret.Labels[capr.ControlPlaneRoleLabel] == "true" {
		r |= LeaderRoleControlPlane
	}
	return r
}

// LeaderCandidate is a single machine-plan secret prepared for election. Adapters produce a
// candidate per secret with Eligible/Init populated from adapter-specific lookups (CAPI machine
// state, v3.Node state, server-args parsing, etc.). The shared electLeader function then applies
// the deterministic preference order.
type LeaderCandidate struct {
	// Secret is the machine-plan secret backing this candidate.
	Secret *corev1.Secret
	// Eligible is false if the candidate must not be picked (e.g., backing machine is being
	// deleted). Ineligible candidates are removed from consideration before tiering.
	Eligible bool
	// Init indicates this candidate is the cluster's init node — the canonical first etcd member.
	// Init candidates always outrank non-init candidates that hold the same roles.
	Init bool
}

// electLeader picks the most suitable LeaderCandidate for the given role(s). Returns nil if no
// eligible candidate holds all requested roles.
//
// Preference is a deterministic three-tier order:
//  1. Init candidate (Init==true) holding all requested roles.
//  2. Candidate whose role set EXACTLY matches the requested roles (least overhead).
//  3. Candidate whose role set is a strict superset of the requested roles.
//
// Within each tier, candidates are sorted lexicographically by secret name. Calling electLeader
// with the same candidates always returns the same secret — no flapping when state changes in
// ways that don't affect the requested role.
func electLeader(role LeaderRole, candidates []LeaderCandidate) *corev1.Secret {
	type ranked struct {
		secret *corev1.Secret
		tier   int
	}

	var ranks []ranked
	for _, c := range candidates {
		if !c.Eligible || c.Secret == nil {
			continue
		}
		cRole := secretRoleSet(c.Secret)
		// Must hold every requested role.
		if cRole&role != role {
			continue
		}
		var tier int
		switch {
		case c.Init:
			tier = 0
		case cRole == role:
			tier = 1
		default:
			tier = 2
		}
		ranks = append(ranks, ranked{secret: c.Secret, tier: tier})
	}
	if len(ranks) == 0 {
		return nil
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].tier != ranks[j].tier {
			return ranks[i].tier < ranks[j].tier
		}
		return ranks[i].secret.Name < ranks[j].secret.Name
	})
	return ranks[0].secret
}
