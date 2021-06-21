package managedos

import (
	"strings"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func objects(mos *provv1.ManagedOS) []runtime.Object {
	concurrency := int64(1)
	if mos.Spec.Concurrency != nil {
		concurrency = *mos.Spec.Concurrency
	}

	cordon := true
	if mos.Spec.Cordon != nil {
		cordon = *mos.Spec.Cordon
	}

	image := strings.SplitN(mos.Spec.OSImage, ":", 2)
	version := "latest"
	if len(image) == 2 {
		version = image[1]
	}

	selector := mos.Spec.NodeSelector
	if selector == nil {
		selector = &metav1.LabelSelector{}
	}

	return []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "os-upgrader",
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"update", "get", "list", "watch", "patch"},
				APIGroups: []string{""},
				Resources: []string{"nodes"},
			}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "os-upgrader",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "os-upgrader",
				Namespace: namespaces.System,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "os-upgrader",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "os-upgrader",
				Namespace: namespaces.System,
			},
		},
		&upgradev1.Plan{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Plan",
				APIVersion: "upgrade.cattle.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "os-upgrader",
				Namespace: namespaces.System,
			},
			Spec: upgradev1.PlanSpec{
				Concurrency: concurrency,
				Version:     version,
				Tolerations: []corev1.Toleration{{
					Operator: corev1.TolerationOpExists,
				}},
				ServiceAccountName: "os-upgrader",
				NodeSelector:       selector,
				Cordon:             cordon,
				Drain:              mos.Spec.Drain,
				Prepare:            mos.Spec.Prepare,
				Upgrade: &upgradev1.ContainerSpec{
					Image: settings.PrefixPrivateRegistry(image[0]),
					Command: []string{
						"/usr/sbin/suc-upgrade",
					},
				},
			},
		},
	}
}
