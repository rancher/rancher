package utils

import (
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/image"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateFluentd(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface, namespace, dockerRootDir string) (err error) {
	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = removeDeamonset(ds, sa, rb, loggingconfig.FluentdName); err != nil {
				logrus.Error("recycle fluentd daemonset failed", err)
			}
		}
	}()

	serviceAccount := newServiceAccount(loggingconfig.FluentdName, namespace)
	_, err = sa.Create(serviceAccount)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	projectRoleBind := newRoleBinding(loggingconfig.FluentdName, namespace)
	_, err = rb.Create(projectRoleBind)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonset := NewFluentdDaemonset(loggingconfig.FluentdName, namespace, dockerRootDir)
	_, err = ds.Create(daemonset)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func CreateLogAggregator(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface, driverDir, namespace string) (err error) {
	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = removeDeamonset(ds, sa, rb, loggingconfig.LogAggregatorName); err != nil {
				logrus.Error("recycle log-aggregator daemonset failed", err)
			}
		}
	}()

	serviceAccount := newServiceAccount(loggingconfig.LogAggregatorName, namespace)
	_, err = sa.Create(serviceAccount)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	projectRoleBind := newRoleBinding(loggingconfig.LogAggregatorName, namespace)
	_, err = rb.Create(projectRoleBind)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonset := NewLogAggregatorDaemonset(loggingconfig.LogAggregatorName, namespace, driverDir)
	_, err = ds.Create(daemonset)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func removeDeamonset(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface, name string) error {
	deleteOp := metav1.DeletePropagationBackground
	var errgrp errgroup.Group
	errgrp.Go(func() error {
		return ds.Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deleteOp})
	})

	errgrp.Go(func() error {
		return sa.Delete(name, &metav1.DeleteOptions{})
	})

	errgrp.Go(func() error {
		return rb.Delete(name, &metav1.DeleteOptions{})
	})

	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func newServiceAccount(name, namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newRoleBinding(name, namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func NewFluentdDaemonset(name, namespace, dockerRootDir string) *v1beta2.DaemonSet {
	privileged := true
	terminationGracePeriodSeconds := int64(30)

	if dockerRootDir == "" {
		dockerRootDir = "/var/lib/docker"
	}
	dockerRootContainers := dockerRootDir + "/containers"

	logVolMounts, logVols := buildHostPathVolumes(map[string][]string{
		"varlibdockercontainers": []string{dockerRootContainers, dockerRootContainers},
		"varlogcontainers":       []string{"/var/log/containers", "/var/log/containers"},
		"varlogpods":             []string{"/var/log/pods", "/var/log/pods"},
		"rkelog":                 []string{"/var/lib/rancher/rke/log", "/var/lib/rancher/rke/log"},
		"customlog":              []string{"/var/lib/rancher/log-volumes", "/var/lib/rancher/log-volumes"},
		"fluentdlog":             []string{"/fluentd/etc/log", "/var/lib/rancher/fluentd/log"},
	})

	configVolMounts, configVols := buildConfigMapVolumes(map[string][]string{
		"clusterlogging": []string{"/fluentd/etc/config/cluster", loggingconfig.ClusterLoggingName, "cluster.conf", "cluster.conf"},
		"projectlogging": []string{"/fluentd/etc/config/project", loggingconfig.ProjectLoggingName, "project.conf", "project.conf"},
	})

	customConfigVolMounts, customConfigVols := buildHostPathVolumes(map[string][]string{
		"clustercustomlogconfig": []string{"/fluentd/etc/config/custom/cluster", "/var/lib/rancher/fluentd/etc/config/custom/cluster"},
		"projectcustomlogconfig": []string{"/fluentd/etc/config/custom/project", "/var/lib/rancher/fluentd/etc/config/custom/project"},
	})

	sslVolMounts, sslVols := buildSecretVolumes(map[string][]string{
		loggingconfig.SSLSecretName: []string{"/fluentd/etc/ssl", loggingconfig.SSLSecretName},
	})

	allConfigVolMounts, allConfigVols := append(configVolMounts, customConfigVolMounts...), append(configVols, customConfigVols...)
	allVolMounts, allVols := append(append(allConfigVolMounts, logVolMounts...), sslVolMounts...), append(append(allConfigVols, logVols...), sslVols...)

	return &v1beta2.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.FluentdName,
			},
		},
		Spec: v1beta2.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					loggingconfig.LabelK8sApp: loggingconfig.FluentdName,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						loggingconfig.LabelK8sApp: loggingconfig.FluentdName,
					},
				},
				Spec: v1.PodSpec{
					Tolerations: []v1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: v1.TaintEffectNoSchedule,
						},
						{
							Key:    "node-role.kubernetes.io/etcd",
							Value:  "true",
							Effect: v1.TaintEffectNoExecute,
						},
						{
							Key:    "node-role.kubernetes.io/controlplane",
							Value:  "true",
							Effect: v1.TaintEffectNoSchedule,
						},
					},
					Containers: []v1.Container{
						{
							Name:    loggingconfig.FluentdHelperName,
							Image:   image.Resolve(v3.ToolsSystemImages.LoggingSystemImages.FluentdHelper),
							Command: []string{"fluentd-helper"},
							Args: []string{
								"--watched-file-list", "/fluentd/etc/config/cluster", "--watched-file-list", "/fluentd/etc/config/project",
								"--watched-file-list", "/fluentd/etc/config/custom/cluster", "--watched-file-list", "/fluentd/etc/config/custom/project",
								"--watched-file-list", "/fluentd/etc/ssl",
							},
							ImagePullPolicy: v1.PullAlways,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: append(allConfigVolMounts, sslVolMounts...),
						},
						{
							Name:            loggingconfig.FluentdName,
							Image:           image.Resolve(v3.ToolsSystemImages.LoggingSystemImages.Fluentd),
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"fluentd"},
							Args:            []string{"-c", "/fluentd/etc/fluent.conf"},
							VolumeMounts:    allVolMounts,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
					ServiceAccountName:            loggingconfig.FluentdName,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Volumes: allVols,
				},
			},
		},
	}
}

func NewLogAggregatorDaemonset(name, namespace, driverDir string) *v1beta2.DaemonSet {
	privileged := true
	return &v1beta2.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.LogAggregatorName,
			},
		},
		Spec: v1beta2.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					loggingconfig.LabelK8sApp: loggingconfig.LogAggregatorName,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						loggingconfig.LabelK8sApp: loggingconfig.LogAggregatorName,
					},
				},
				Spec: v1.PodSpec{
					Tolerations: []v1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: v1.TaintEffectNoSchedule,
						},
						{
							Key:    "node-role.kubernetes.io/etcd",
							Value:  "true",
							Effect: v1.TaintEffectNoExecute,
						},
						{
							Key:    "node-role.kubernetes.io/controlplane",
							Value:  "true",
							Effect: v1.TaintEffectNoSchedule,
						},
					},
					Containers: []v1.Container{
						{
							Name:            loggingconfig.LogAggregatorName,
							Image:           image.Resolve(v3.ToolsSystemImages.LoggingSystemImages.LogAggregatorFlexVolumeDriver),
							ImagePullPolicy: v1.PullAlways,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "flexvolume-driver",
									MountPath: "/flexmnt",
								},
							},
						},
					},
					ServiceAccountName: loggingconfig.LogAggregatorName,
					Volumes: []v1.Volume{
						{
							Name: "flexvolume-driver",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: driverDir,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getDriverDir(driverName string) string {
	switch driverName {
	case v3.ClusterDriverRKE:
		return "/var/lib/kubelet/volumeplugins"
	case loggingconfig.GoogleKubernetesEngine:
		return "/home/kubernetes/flexvolume"
	default:
		return "/usr/libexec/kubernetes/kubelet-plugins/volume/exec"
	}
}

func buildHostPathVolumes(mounts map[string][]string) (vms []v1.VolumeMount, vs []v1.Volume) {
	for name, value := range mounts {
		vms = append(vms, v1.VolumeMount{
			Name:      name,
			MountPath: value[0],
		})
		vs = append(vs, v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: value[1],
				},
			},
		})
	}
	return
}

func buildConfigMapVolumes(mounts map[string][]string) (vms []v1.VolumeMount, vs []v1.Volume) {
	for name, value := range mounts {
		vms = append(vms, v1.VolumeMount{
			Name:      name,
			MountPath: value[0],
		})
		vs = append(vs, v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: value[1],
					},
					Items: []v1.KeyToPath{
						{
							Key:  value[2],
							Path: value[3],
						},
					},
				},
			},
		})
	}
	return
}

func buildSecretVolumes(mounts map[string][]string) (vms []v1.VolumeMount, vs []v1.Volume) {
	for name, value := range mounts {
		vms = append(vms, v1.VolumeMount{
			Name:      name,
			MountPath: value[0],
		})
		vs = append(vs, v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{

				Secret: &v1.SecretVolumeSource{
					SecretName: value[1],
				},
			},
		})
	}
	return
}
