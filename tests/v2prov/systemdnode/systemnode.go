package systemdnode

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New is used to create a new instance of systemd-node as a pod in a Kubernetes cluster. Notably, this is used for the
// v2prov integration tests for custom clusters to simulate custom cluster nodes. If extraDirs is defined, it expects a slice of elements of format "host-source:pod-mount".
func New(clients *clients.Clients, namespace, script string, labels map[string]string, extraDirs []string) (*corev1.Pod, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-node-data-",
		},
		Data: map[string]string{
			"user-data": script,
		},
	}
	cm, err := clients.Core.ConfigMap().Create(cm)
	if err != nil {
		return nil, err
	}

	// We are providing the following files/configmap in order to force K3s/RKE2 to use the cgroupfs cgroup driver,
	// rather than systemd.
	cmConfigure := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-node-configure-systemd-",
		},
		Data: map[string]string{
			"disable": `INVOCATION_ID=
`},
	}
	cmConfigure, err = clients.Core.ConfigMap().Create(cmConfigure)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-node-",
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "seed",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cm.Name,
							},
						},
					},
				},
				{
					Name: "systemd",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cmConfigure.Name,
							},
						},
					},
				},
			},
			Containers: []corev1.Container{{
				Name:  "container",
				Image: defaults.PodTestImage,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &[]bool{true}[0],
				},
				Stdin:     true,
				StdinOnce: true,
				TTY:       true,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "seed",
						MountPath: "/var/lib/cloud/seed/nocloud/user-data",
						SubPath:   "user-data",
					},
					{ // We have to set invocation disabling on the rancher-system-agent because it runs rke2/k3s server on restore and this has cgroup issues
						Name:      "systemd",
						MountPath: "/etc/default/rancher-system-agent",
						SubPath:   "disable",
					},
					{
						Name:      "systemd",
						MountPath: "/etc/default/rke2-server",
						SubPath:   "disable",
					},
					{
						Name:      "systemd",
						MountPath: "/etc/default/rke2-agent",
						SubPath:   "disable",
					},
					{
						Name:      "systemd",
						MountPath: "/etc/default/k3s",
						SubPath:   "disable",
					},
					{
						Name:      "systemd",
						MountPath: "/etc/default/k3s-agent",
						SubPath:   "disable",
					},
				},
			}},
			AutomountServiceAccountToken: new(bool),
		},
	}

	for i, v := range extraDirs {
		hostSource, mountPath, ok := strings.Cut(v, ":")
		if !ok {
			// TODO: log
			continue
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: fmt.Sprintf("extra-directory-%d", i),
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: hostSource,
					Type: &[]corev1.HostPathType{corev1.HostPathDirectoryOrCreate}[0],
				},
			},
		})

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf("extra-directory-%d", i),
			MountPath: mountPath,
		})

	}

	pod, err = clients.Core.Pod().Create(pod)
	if err != nil {
		return nil, err
	}
	clients.OnClose(func() {
		clients.Core.Pod().Delete(pod.Namespace, pod.Name, nil)
	})

	cm.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       pod.Name,
		UID:        pod.UID,
	}}

	_, err = clients.Core.ConfigMap().Update(cm)
	return pod, err
}
