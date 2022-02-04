package machineprovision

import (
	"strconv"

	name2 "github.com/rancher/wrangler/pkg/name"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	InfraMachineGroup   = "rke.cattle.io/infra-machine-group"
	InfraMachineVersion = "rke.cattle.io/infra-machine-version"
	InfraMachineKind    = "rke.cattle.io/infra-machine-kind"
	InfraMachineName    = "rke.cattle.io/infra-machine-name"
	InfraJobRemove      = "rke.cattle.io/infra-remove"

	pathToMachineFiles = "/path/to/machine/files"
)

var (
	oneThousand int64 = 1000
)

func GetJobName(name string) string {
	return name2.SafeConcatName(name, "machine", "provision")
}

func objects(ready bool, args driverArgs) []runtime.Object {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.StateSecretName,
			Namespace: args.MachineNamespace,
		},
		Type: "rke.cattle.io/machine-state",
	}

	if ready {
		// If the machine is ready, then we only need the secret.
		return []runtime.Object{secret}
	}

	volumes := make([]corev1.Volume, 0, 2)
	volumeMounts := make([]corev1.VolumeMount, 0, 2)
	saName := GetJobName(args.MachineName)

	if args.BootstrapRequired {
		volumes = append(volumes, corev1.Volume{
			Name: "bootstrap",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  args.BootstrapSecretName,
					DefaultMode: &[]int32{0777}[0],
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "bootstrap",
			MountPath: "/run/secrets/machine",
			ReadOnly:  true,
		})
	}

	if args.FilesSecret != nil {
		if args.FilesSecret.Name == "" {
			args.FilesSecret.Name = saName
			args.FilesSecret.Namespace = args.MachineNamespace
		}

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "machine-files",
			ReadOnly:  true,
			MountPath: pathToMachineFiles,
		})

		keysToPaths := make([]corev1.KeyToPath, 0, len(args.FilesSecret.Data))
		for file := range args.FilesSecret.Data {
			keysToPaths = append(keysToPaths, corev1.KeyToPath{Key: file, Path: file})
		}
		volumes = append(volumes, corev1.Volume{
			Name: "machine-files",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  args.FilesSecret.Name,
					Items:       keysToPaths,
					DefaultMode: &[]int32{0644}[0],
				},
			},
		})
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: args.MachineNamespace,
		},
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: args.MachineNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get", "update"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secret.Name},
			},
		},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: args.MachineNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: args.MachineNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     saName,
		},
	}
	rb2 := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name2.SafeConcatName(saName, "extension"),
			Namespace: args.MachineNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: args.MachineNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "rke2-machine-provisioner",
		},
	}

	labels := map[string]string{
		InfraMachineGroup:   args.MachineGVK.Group,
		InfraMachineVersion: args.MachineGVK.Version,
		InfraMachineKind:    args.MachineGVK.Kind,
		InfraMachineName:    args.MachineName,
		InfraJobRemove:      strconv.FormatBool(!args.BootstrapRequired),
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: args.MachineNamespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &args.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: append(volumes, corev1.Volume{
						Name: "tls-ca-additional-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  "tls-ca-additional",
								DefaultMode: &[]int32{0444}[0],
								Optional:    &[]bool{true}[0],
							},
						},
					}),
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name: "machine",
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  &oneThousand,
								RunAsGroup: &oneThousand,
							},
							Image:           args.ImageName,
							ImagePullPolicy: args.ImagePullPolicy,
							Args:            args.Args,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: args.EnvSecret.Name,
										},
									},
								},
							},
							VolumeMounts: append(volumeMounts, corev1.VolumeMount{
								Name:      "tls-ca-additional-volume",
								ReadOnly:  true,
								MountPath: "/etc/ssl/certs/ca-additional.pem",
								SubPath:   "ca-additional.pem",
							}),
						},
					},
					ServiceAccountName: saName,
				},
			},
		},
	}

	return []runtime.Object{
		args.EnvSecret,
		secret,
		sa,
		role,
		rb,
		args.FilesSecret,
		rb2,
		job,
	}
}
