package operations

import (
	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
)

type Filter func(secret *corev1.Secret) bool

func IsEtcd(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.EtcdRoleLabel] == "true"
}

func IsControlPlane(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.ControlPlaneRoleLabel] == "true"
}

func IsWindows(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.CattleOSLabel] == "windows"
}

func And(l, r Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return l(secret) && r(secret)
	}
}

func Or(l, r Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return l(secret) || r(secret)
	}
}

func Not(filter Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return !filter(secret)
	}
}
