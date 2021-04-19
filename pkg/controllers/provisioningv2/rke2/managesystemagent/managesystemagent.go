package managesystemagent

import (
	"context"
	"encoding/json"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	workerAffinity = corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "node-role.kubernetes.io/etcd",
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"true"},
					},
					{
						Key:      "node-role.kubernetes.io/control-plane",
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"true"},
					},
					{
						Key:      "node-role.kubernetes.io/master",
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"true"},
					},
					{
						Key:      "beta.kubernetes.io/os",
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"windows"},
					},
				},
			}},
		},
	}
	controlPlaneAffinity = corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "node-role.kubernetes.io/etcd",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"true"},
					}},
				},
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "node-role.kubernetes.io/control-plane",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"true"},
					}},
				},
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "node-role.kubernetes.io/master",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"true"},
					}},
				},
			},
		},
	}
)

type handler struct{}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{}
	rocontrollers.RegisterClusterGeneratingHandler(ctx, clients.Provisioning.Cluster(),
		clients.Apply.
			WithSetOwnerReference(false, false).
			WithCacheTypes(clients.Fleet.Bundle(),
				clients.Provisioning.Cluster()),
		"", "manage-system-agent", h.OnChange, nil)
}

func (h *handler) OnChange(cluster *rancherv1.Cluster, status rancherv1.ClusterStatus) ([]runtime.Object, rancherv1.ClusterStatus, error) {
	if cluster.Spec.RKEConfig == nil {
		return nil, status, nil
	}

	bundle := &v1alpha1.Bundle{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      name.SafeConcatName(cluster.Name, "managed", "system", "agent"),
		},
		Spec: v1alpha1.BundleSpec{
			Resources: []v1alpha1.BundleResource{
				{
					Name:    "one.yaml",
					Content: installer("true", &workerAffinity),
				},
				{
					Name:    "two.yaml",
					Content: installer("false", &controlPlaneAffinity),
				},
			},
			Targets: []v1alpha1.BundleTarget{
				{
					ClusterSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "metadata.name",
								Operator: metav1.LabelSelectorOpIn,
								Values: []string{
									cluster.Name,
								},
							},
							{
								Key:      "provisioning.cattle.io/unmanaged-system-agent",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
						},
					},
				},
			},
		},
	}

	return []runtime.Object{
		bundle,
	}, status, nil
}

func installer(agentEnvValue string, affinity *corev1.NodeAffinity) string {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-agent-upgrader-worker",
			Namespace: namespaces.System,
		},
		Spec: v1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "system-agent-upgrader",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "system-agent-upgrader",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					}},
					HostIPC:     true,
					HostPID:     true,
					HostNetwork: true,
					DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
					Tolerations: []corev1.Toleration{{
						Operator: corev1.TolerationOpExists,
					}},
					Containers: []corev1.Container{{
						Name:  "installer",
						Image: settings.PrefixPrivateRegistry(settings.SystemAgentUpgradeImage.Get()),
						Env: []corev1.EnvVar{{
							Name:  "CATTLE_ROLE_WORKER",
							Value: agentEnvValue,
						}},
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "steve-aggregation",
								},
							},
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "host",
							MountPath: "/host",
						}},
						SecurityContext: &corev1.SecurityContext{
							Privileged: &[]bool{true}[0],
						},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Affinity: &corev1.Affinity{
						NodeAffinity: affinity,
					},
				},
			},
		},
	}
	file, err := json.Marshal(ds)
	if err != nil {
		panic(err)
	}
	return string(file)
}
