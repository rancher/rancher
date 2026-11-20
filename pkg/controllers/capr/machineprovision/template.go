package machineprovision

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	name2 "github.com/rancher/wrangler/v3/pkg/name"
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
	CapiMachineName     = "rke.cattle.io/capi-machine-name"

	pathToMachineFiles = "/path/to/machine/files"
	sslCertDir         = "/etc/rancher/ssl"

	driverDownloadURLFlag  = "--driver-download-url="
	driverDownloadHashFlag = "--driver-hash="
	driverDownloadURLEnv   = "CATTLE_DRIVER_DOWNLOAD_URL"
	driverDownloadHashEnv  = "CATTLE_DRIVER_DOWNLOAD_HASH"
	driverBinaryPrefix     = "docker-machine-driver-"
	machineEntrypoint      = "/usr/local/bin/entrypoint.sh"
)

var (
	oneThousand int64 = 1000
)

func GetJobName(name string) string {
	return name2.SafeConcatName(name, "machine", "provision")
}

// driverDownload describes how the driver binary should be fetched inside the
// machine provision pod. Drivers served from Rancher's own assets are already
// extracted binaries, so they can be downloaded directly instead of going
// through the machine image's download_driver.sh script. That script relies on
// sed and grep, which are not present in all rancher/machine images.
type driverDownload struct {
	args       []string
	url        string
	hash       string
	binaryName string
	direct     bool
}

func getDriverDownload(args driverArgs) driverDownload {
	var url, hash string
	remaining := make([]string, 0, len(args.Args))
	for _, arg := range args.Args {
		switch {
		case strings.HasPrefix(arg, driverDownloadURLFlag):
			url = strings.TrimPrefix(arg, driverDownloadURLFlag)
		case strings.HasPrefix(arg, driverDownloadHashFlag):
			hash = strings.TrimPrefix(arg, driverDownloadHashFlag)
		default:
			remaining = append(remaining, arg)
		}
	}

	if url == "" || !strings.Contains(url, "/assets/"+driverBinaryPrefix) {
		return driverDownload{args: args.Args}
	}

	binaryName := args.DriverName
	if !strings.HasPrefix(binaryName, driverBinaryPrefix) {
		binaryName = driverBinaryPrefix + binaryName
	}

	return driverDownload{
		args:       remaining,
		url:        url,
		hash:       hash,
		binaryName: binaryName,
		direct:     true,
	}
}

func (d driverDownload) command() []string {
	if !d.direct {
		return nil
	}

	dest := "/usr/local/bin/" + d.binaryName
	script := fmt.Sprintf(`set -e
if [ -n "${SSL_CERT_DIR}" ]; then
  ln -s /etc/ssl/certs/* "${SSL_CERT_DIR}" 2>/dev/null || true
fi
if command -v c_rehash >/dev/null 2>&1; then
  c_rehash >/dev/null 2>&1 || true
fi
curl -fsSL -o %q "$%s"
if [ -n "$%s" ]; then
  printf '%%s  %%s\n' "$%s" %q | sha256sum -c -
fi
chmod 0755 %q
exec %s "$@"
`, dest, driverDownloadURLEnv, driverDownloadHashEnv, driverDownloadHashEnv, dest, dest, machineEntrypoint)

	return []string{"/bin/sh", "-c", script}
}

func (d driverDownload) env() []corev1.EnvVar {
	if !d.direct {
		return nil
	}

	return []corev1.EnvVar{
		{Name: driverDownloadURLEnv, Value: d.url},
		{Name: driverDownloadHashEnv, Value: d.hash},
	}
}

func objects(ready bool, args driverArgs) []runtime.Object {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.StateSecretName,
			Namespace: args.MachineNamespace,
			Labels: map[string]string{
				capr.BackupLabel: "true",
			},
		},
		Type: "rke.cattle.io/machine-state",
	}

	if ready {
		// If the machine is ready, then we only need the secret.
		return []runtime.Object{secret}
	}

	volumes := make([]corev1.Volume, 0)
	volumeMounts := make([]corev1.VolumeMount, 0)
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

		// Because of the way apply works, it must be ensured that the keysToPaths slice is always in the same order.
		sort.Slice(keysToPaths, func(i, j int) bool {
			return keysToPaths[i].Key < keysToPaths[j].Key
		})

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

	if args.CertsSecret != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "machine-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  args.CertsSecret.Name,
					DefaultMode: &[]int32{0644}[0],
				},
			},
		})
		for key := range args.CertsSecret.Data {
			// Setting one volume mount for each cert ensures that the directory remains writable in the container.
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "machine-certs",
				ReadOnly:  true,
				MountPath: sslCertDir + "/" + key,
				SubPath:   key,
			})
		}
	}

	// Because of the way apply works, it must be ensured that the volumeMounts always appear in the same order.
	sort.Slice(volumeMounts, func(i, j int) bool {
		return volumeMounts[i].MountPath < volumeMounts[j].MountPath
	})

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
		CapiMachineName:     args.CapiMachineName,
	}

	download := getDriverDownload(args)
	machineArgs := args.Args
	if download.direct {
		machineArgs = append([]string{"machine-provision"}, download.args...)
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
					Volumes:       volumes,
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
							Command:         download.command(),
							Args:            machineArgs,
							Env:             download.env(),
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: args.EnvSecret.Name,
										},
									},
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					ServiceAccountName: saName,
				},
			},
		},
	}

	if len(args.ImagePullSecrets) > 0 {
		job.Spec.Template.Spec.ImagePullSecrets = args.ImagePullSecrets
	}

	return []runtime.Object{
		args.EnvSecret,
		secret,
		sa,
		role,
		rb,
		args.FilesSecret,
		args.CertsSecret,
		rb2,
		job,
	}
}
