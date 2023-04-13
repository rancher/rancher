package systemdnode

import (
	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New is used to create a new instance of systemd-node as a pod in a Kubernetes cluster. Notably, this is used for the
// v2prov integration tests for custom clusters to simulate custom cluster nodes.
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

	// We are providing the following files/configmap in order to force K3s/RKE2 to use the cgroupfs cgroup driver,
	// rather than systemd.
	// We must also drop-in replace the type of service for systemd as we are clearing the notify socket which would
	// normally cause systemd to hang waiting for the unit to activate. Eventually, when
	// https://github.com/rancher/rke2/issues/3240 is resolved, we should be able to roll back this workaround. If the
	// linked github issue is resolved, we should roll back this change here as well as in
	// tests/integration/pkg/nodeconfig.
	cmConfigure := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-node-configure-systemd-",
		},
		Data: map[string]string{
			"dropin": `[Service]
Type=exec
`,
			"disable": `NOTIFY_SOCKET=
INVOCATION_ID=
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
					{
						Name:      "systemd",
						MountPath: "/usr/local/lib/systemd/system/rke2-server.service.d/10-delegate.conf",
						SubPath:   "dropin",
					},
					{
						Name:      "systemd",
						MountPath: "/usr/local/lib/systemd/system/rke2-agent.service.d/10-delegate.conf",
						SubPath:   "dropin",
					},
					{
						Name:      "systemd",
						MountPath: "/usr/local/lib/systemd/system/k3s.service.d/10-delegate.conf",
						SubPath:   "dropin",
					},
					{
						Name:      "systemd",
						MountPath: "/usr/local/lib/systemd/system/k3s-agent.service.d/10-delegate.conf",
						SubPath:   "dropin",
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
