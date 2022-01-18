package systemdnode

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func New(clients *clients.Clients, namespace, script string) (*corev1.Pod, error) {
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

	imagesPath := fmt.Sprintf("/var/lib/rancher/%s/agent/images", os.Getenv("DIST"))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-node-",
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
					Name: "image-assets",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/image-assets",
						},
					},
				},
				{
					Name: "agent-images",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{{
				Name:  "copy-image-assets",
				Image: defaults.PodTestImage,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &[]bool{true}[0],
				},
				Command: []string{
					"/bin/sh",
				},
				Args: []string{
					"-c",
					"cp -rf /image-assets/* /image-assets-destination",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "image-assets",
						MountPath: "/image-assets",
					},
					{
						Name:      "agent-images",
						MountPath: "/image-assets-destination",
					},
				},
			}},
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
					{
						Name:      "agent-images",
						MountPath: imagesPath,
					},
				},
			}},
			AutomountServiceAccountToken: new(bool),
		},
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
