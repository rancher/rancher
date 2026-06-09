package operations

import (
	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
)

// Filter is a predicate over machine-plan secrets. It returns true when the secret matches the
// filter criteria and false otherwise. Filters are used in adapter implementations and operation
// controllers to classify secrets by role or OS before dispatching plans.
//
// Filters can be composed via And, Or, and Not for more complex predicates:
//
//	etcdNotWindows := And(IsEtcd, Not(IsWindows))
type Filter func(secret *corev1.Secret) bool

// IsEtcd returns true when the secret carries the rke.cattle.io/etcd-role="true" label. This
// label is set by the planner (for CAPR clusters) or the unmanaged controller (for imported
// clusters) and signals that the downstream node runs etcd. Etcd nodes receive snapshot, restore,
// and cluster-reset plans.
func IsEtcd(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.EtcdRoleLabel] == "true"
}

// IsControlPlane returns true when the secret carries the rke.cattle.io/controlplane-role="true"
// label. Control plane nodes run kube-apiserver, kube-controller-manager, and kube-scheduler.
// Some plans (e.g., drain, cordon) target only control plane nodes; others target both etcd and
// control plane.
func IsControlPlane(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.ControlPlaneRoleLabel] == "true"
}

// IsWindows returns true when the secret carries the cattle.io/os="windows" label. Windows nodes
// require different plan instructions (PowerShell vs bash, different filesystem paths, etc.) and
// are often excluded from etcd/control-plane plans (Windows can only run worker nodes in a
// mixed-OS cluster).
func IsWindows(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.CattleOSLabel] == "windows"
}

func IsInitNode(s *corev1.Secret) bool {
	return s.Labels[capr.InitNodeLabel] == "true"
}

// And returns a Filter that requires both l and r to return true. Short-circuits on the first
// false — if l(secret) is false, r is not evaluated.
func And(l, r Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return l(secret) && r(secret)
	}
}

// Or returns a Filter that requires at least one of l or r to return true. Short-circuits on the
// first true — if l(secret) is true, r is not evaluated.
func Or(l, r Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return l(secret) || r(secret)
	}
}

// Not returns a Filter that negates the input filter — true becomes false and vice versa.
func Not(filter Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return !filter(secret)
	}
}
