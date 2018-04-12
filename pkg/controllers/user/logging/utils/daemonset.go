package utils

import (
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func CreateFluentd(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface, namespace string) (err error) {
	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = removeDeamonset(ds, sa, rb, loggingconfig.FluentdName); err != nil {
				logrus.Error("recycle daemonset failed", err)
			}
		}
	}()

	serviceAccount := newServiceAccount(loggingconfig.FluentdName, namespace)
	serviceAccount, err = sa.Create(serviceAccount)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	projectRoleBind := newRoleBinding(loggingconfig.FluentdName, namespace)
	projectRoleBind, err = rb.Create(projectRoleBind)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonset := newFluentdDaemonset(loggingconfig.FluentdName, namespace, loggingconfig.FluentdName)
	daemonset, err = ds.Create(daemonset)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func CreateLogAggregator(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface, cl v3.ClusterLister, clusterName, namespace string) (err error) {
	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = removeDeamonset(ds, sa, rb, loggingconfig.LogAggregatorName); err != nil {
				logrus.Error("recycle daemonset failed", err)
			}
		}
	}()

	serviceAccount := newServiceAccount(loggingconfig.LogAggregatorName, namespace)
	serviceAccount, err = sa.Create(serviceAccount)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	projectRoleBind := newRoleBinding(loggingconfig.LogAggregatorName, namespace)
	projectRoleBind, err = rb.Create(projectRoleBind)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	cluster, err := cl.Get("", clusterName)
	if err != nil {
		return err
	}
	driverDir := getDriverDir(cluster.Status.Driver)
	daemonset := newLogAggregatorDaemonset(loggingconfig.LogAggregatorName, namespace, driverDir)
	daemonset, err = ds.Create(daemonset)
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

func newFluentdDaemonset(name, namespace, clusterName string) *v1beta2.DaemonSet {
	privileged := true
	terminationGracePeriodSeconds := int64(30)
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
					Tolerations: []v1.Toleration{{
						Key:    "node-role.kubernetes.io/master",
						Effect: v1.TaintEffectNoSchedule,
					}},
					Containers: []v1.Container{
						{
							Name:            loggingconfig.FluentdHelperName,
							Image:           v3.ToolsSystemImages.LoggingSystemImages.FluentdHelper,
							Command:         []string{"fluentd-helper"},
							Args:            []string{"--watched-file-list", "/fluentd/etc/config/cluster", "--watched-file-list", "/fluentd/etc/config/project", "--watched-file-list", "/fluentd/etc/config/custom/cluster", "--watched-file-list", "/fluentd/etc/config/custom/project"},
							ImagePullPolicy: v1.PullAlways,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "clusterlogging",
									MountPath: "/fluentd/etc/config/cluster",
								},
								{
									Name:      "projectlogging",
									MountPath: "/fluentd/etc/config/project",
								},
								{
									Name:      "clustercustomlogconfig",
									MountPath: "/fluentd/etc/config/custom/cluster",
								},
								{
									Name:      "projectcustomlogconfig",
									MountPath: "/fluentd/etc/config/custom/project",
								},
							},
						},
						{
							Name:            loggingconfig.FluentdName,
							Image:           v3.ToolsSystemImages.LoggingSystemImages.Fluentd,
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"fluentd"},
							Args:            []string{"-c", "/fluentd/etc/fluent.conf"},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "varlibdockercontainers",
									MountPath: "/var/lib/docker/containers",
								},
								{
									Name:      "varlogcontainers",
									MountPath: "/var/log/containers",
								},
								{
									Name:      "varlogpods",
									MountPath: "/var/log/pods",
								},
								{
									Name:      "fluentdlog",
									MountPath: "/fluentd/etc/log",
								},
								{
									Name:      "rkelog",
									MountPath: "/var/lib/rancher/rke/log",
								},
								{
									Name:      "clusterlogging",
									MountPath: "/fluentd/etc/config/cluster",
								},
								{
									Name:      "projectlogging",
									MountPath: "/fluentd/etc/config/project",
								},
								{
									Name:      "customlog",
									MountPath: "/var/log/rancher-log-volumes",
								},
								{
									Name:      "clustercustomlogconfig",
									MountPath: "/fluentd/etc/config/custom/cluster",
								},
								{
									Name:      "projectcustomlogconfig",
									MountPath: "/fluentd/etc/config/custom/project",
								},
							},
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
					ServiceAccountName:            loggingconfig.FluentdName,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Volumes: []v1.Volume{
						{
							Name: "varlibdockercontainers",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/docker/containers",
								},
							},
						},
						{
							Name: "varlogcontainers",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/log/containers",
								},
							},
						},
						{
							Name: "varlogpods",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/log/pods",
								},
							},
						},
						{
							Name: "rkelog",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/rancher/rke/log",
								},
							},
						},
						{
							Name: "customlog",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/log/rancher-log-volumes",
								},
							},
						},
						{
							Name: "clustercustomlogconfig",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/fluentd/etc/config/custom/cluster",
								},
							},
						},
						{
							Name: "projectcustomlogconfig",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/fluentd/etc/config/custom/project",
								},
							},
						},
						{
							Name: "fluentdlog",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/log/fluentd",
								},
							},
						},
						{
							Name: "clusterlogging",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: loggingconfig.ClusterLoggingName,
									},
									Items: []v1.KeyToPath{
										{
											Key:  "cluster.conf",
											Path: "cluster.conf",
										},
									},
								},
							},
						},
						{
							Name: "projectlogging",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: loggingconfig.ProjectLoggingName,
									},
									Items: []v1.KeyToPath{
										{
											Key:  "project.conf",
											Path: "project.conf",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newLogAggregatorDaemonset(name, namespace, driverDir string) *v1beta2.DaemonSet {
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
					Containers: []v1.Container{
						{
							Name:            loggingconfig.LogAggregatorName,
							Image:           v3.ToolsSystemImages.LoggingSystemImages.LogAggregatorFlexVolumeDriver,
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
