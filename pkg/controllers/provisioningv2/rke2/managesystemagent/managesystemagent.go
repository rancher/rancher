package managesystemagent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	workerSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/etcd",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"true"},
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"true"},
			},
			{
				Key:      "beta.kubernetes.io/os",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"windows"},
			},
		},
	}
	controlPlaneSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/etcd",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"true"},
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
		},
	}
	etcdSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/etcd",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"true"},
			},
		},
	}
	controlPlaneAndEtcdSelector = metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/etcd",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
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
	rocontrollers.RegisterClusterGeneratingHandler(ctx, clients.Provisioning.Cluster(),
		clients.Apply.
			WithSetOwnerReference(false, false).
			WithCacheTypes(clients.Mgmt.ManagedChart(),
				clients.Provisioning.Cluster()),
		"", "manage-system-upgrade-controller", h.OnChangeInstallSUC, nil)
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
			BundleDeploymentOptions: v1alpha1.BundleDeploymentOptions{
				DefaultNamespace: namespaces.System,
			},
			Resources: []v1alpha1.BundleResource{
				{
					Name:    "cp.yaml",
					Content: installer("cp", "false", "false", "true", cluster.Spec.AgentEnvVars, &controlPlaneSelector),
				},
				{
					Name:    "etcd.yaml",
					Content: installer("etcd", "false", "true", "false", cluster.Spec.AgentEnvVars, &etcdSelector),
				},
				{
					Name:    "cp-and-etcd.yaml",
					Content: installer("cp-and-etcd", "false", "true", "true", cluster.Spec.AgentEnvVars, &controlPlaneAndEtcdSelector),
				},
				{
					Name:    "worker.yaml",
					Content: installer("worker", "true", "false", "false", cluster.Spec.AgentEnvVars, &workerSelector),
				},
			},
			Targets: []v1alpha1.BundleTarget{
				{
					ClusterName: cluster.Name,
					ClusterSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
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

func installer(name, worker, etcd, controlPlane string, envs []corev1.EnvVar, selector *metav1.LabelSelector) string {
	image := strings.SplitN(settings.SystemAgentUpgradeImage.Get(), ":", 2)
	version := "latest"
	if len(image) == 2 {
		version = image[1]
	}

	plan := &upgradev1.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Plan",
			APIVersion: "upgrade.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-agent-upgrader-" + name,
			Namespace: namespaces.System,
		},
		Spec: upgradev1.PlanSpec{
			Concurrency: 10,
			Version:     version,
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			}},
			NodeSelector: selector,
			Upgrade: &upgradev1.ContainerSpec{
				Image:   settings.PrefixPrivateRegistry(image[0]),
				Command: nil,
				Args:    nil,
				Env: append(envs, []corev1.EnvVar{
					{
						Name:  "CATTLE_ROLE_WORKER",
						Value: worker,
					},
					{
						Name:  "CATTLE_ROLE_ETCD",
						Value: etcd,
					},
					{
						Name:  "CATTLE_ROLE_CONTROL_PLANE",
						Value: controlPlane,
					},
				}...),
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "steve-aggregation",
						},
					},
				}},
			},
		},
	}

	file, err := json.Marshal(plan)
	if err != nil {
		panic(err)
	}
	return string(file)
}
