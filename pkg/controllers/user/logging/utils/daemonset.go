package utils

import (
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"golang.org/x/sync/errgroup"
	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func CreateFluentd(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface, namespace string) (err error) {
	serviceAccount := newFluentdServiceAccount(loggingconfig.FluentdName, namespace)
	serviceAccount, err = sa.Create(serviceAccount)
	defer func() {
		if err != nil {
			sa.Delete(serviceAccount.Name, &metav1.DeleteOptions{})
		}
	}()
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	projectRoleBind := newFluentdRoleBinding(loggingconfig.FluentdName, namespace)
	projectRoleBind, err = rb.Create(projectRoleBind)
	defer func() {
		if err != nil {
			rb.Delete(projectRoleBind.Name, &metav1.DeleteOptions{})
		}
	}()
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonset := newFluentdDaemonset(loggingconfig.FluentdName, namespace, loggingconfig.FluentdName)
	daemonset, err = ds.Create(daemonset)
	defer func() {
		if err != nil {
			ds.Delete(daemonset.Name, &metav1.DeleteOptions{})
		}
	}()
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func RemoveFluentd(ds rv1beta2.DaemonSetInterface, sa rv1.ServiceAccountInterface, rb rrbacv1.ClusterRoleBindingInterface) error {
	deleteOp := metav1.DeletePropagationBackground
	var errgrp errgroup.Group
	errgrp.Go(func() error {
		return ds.Delete(loggingconfig.FluentdName, &metav1.DeleteOptions{PropagationPolicy: &deleteOp})
	})

	errgrp.Go(func() error {
		return sa.Delete(loggingconfig.FluentdName, &metav1.DeleteOptions{})
	})

	errgrp.Go(func() error {
		return rb.Delete(loggingconfig.FluentdName, &metav1.DeleteOptions{})
	})

	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func newFluentdServiceAccount(name, namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newFluentdRoleBinding(name, namespace string) *rbacv1.ClusterRoleBinding {
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
							Args:            []string{"--watched-file-list", "/fluentd/etc/config/cluster", "--watched-file-list", "/fluentd/etc/config/project"},
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
									Name:      "clusterlogging",
									MountPath: "/fluentd/etc/config/cluster",
								},
								{
									Name:      "projectlogging",
									MountPath: "/fluentd/etc/config/project",
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
