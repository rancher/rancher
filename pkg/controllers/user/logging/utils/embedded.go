package utils

import (
	"github.com/pkg/errors"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	rv1 "github.com/rancher/types/apis/core/v1"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1beta2 "k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func CreateEmbeddedTarget(dep rv1beta2.DeploymentInterface, sa rv1.ServiceAccountInterface, se rv1.ServiceInterface, ro rrbacv1.RoleInterface, rb rrbacv1.RoleBindingInterface, namespace string) error {
	// create es deployment
	_, err := dep.Controller().Lister().Get(loggingconfig.EmbeddedESName, loggingconfig.EmbeddedESName)
	if err != nil {
		logrus.Infof("get embedded es error: %v", err)
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get deployment %s fail", loggingconfig.EmbeddedESName)
		}
		logrus.Infof("get embedded es error is not found")
		// create service account, role and rolebinding
		sc := newESServiceAccount(namespace)
		role := newESRole(namespace)
		roleBind := newESRoleBinding(namespace)

		defer func() {
			if err != nil {
				sa.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
			}
		}()
		_, err = sa.Create(sc)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create service account %s fail", loggingconfig.EmbeddedESName)
		}

		defer func() {
			if err != nil {
				ro.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
			}
		}()
		_, err = ro.Create(role)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedESName)
		}

		defer func() {
			if err != nil {
				rb.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
			}
		}()
		_, err = rb.Create(roleBind)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedESName)
		}

		// create service and deployment
		defer func() {
			if err != nil {
				se.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
			}
		}()
		newService := NewESService(namespace)
		_, err = se.Create(newService)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create service %s fail", loggingconfig.EmbeddedESName)
		}

		defer func() {
			if err != nil {
				dep.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
			}
		}()
		esDeployment := NewESDeployment(namespace)
		_, err = dep.Create(esDeployment)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create deployment %s fail", loggingconfig.EmbeddedESName)
		}
	}

	// create kibana deployment
	_, err = dep.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedKibanaName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "get deployment %s fail", loggingconfig.EmbeddedKibanaName)
		}

		// create service account, role and rolebinding
		sc := newKibanaServiceAccount(namespace)
		role := newKibanaRole(namespace)
		roleBind := newKibanaRoleBinding(namespace)

		defer func() {
			if err != nil {
				sa.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
			}
		}()
		_, err = sa.Create(sc)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create service account  %s fail", loggingconfig.EmbeddedKibanaName)
		}

		defer func() {
			if err != nil {
				ro.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
			}
		}()
		_, err = ro.Create(role)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedKibanaName)
		}

		defer func() {
			if err != nil {
				rb.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
			}
		}()
		_, err = rb.Create(roleBind)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedKibanaName)
		}

		defer func() {
			if err != nil {
				se.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
			}
		}()
		newService := NewKibanaService(namespace)
		_, err = se.Create(newService)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create service %s fail", loggingconfig.EmbeddedKibanaName)
		}

		defer func() {
			if err != nil {
				dep.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
			}
		}()
		kibanaDeployment := NewKibanaDeployment(namespace)
		_, err = dep.Create(kibanaDeployment)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create deployment %s fail", loggingconfig.EmbeddedKibanaName)
		}
	}
	return nil
}

func RemoveEmbeddedTarget(dep rv1beta2.DeploymentInterface, sa rv1.ServiceAccountInterface, se rv1.ServiceInterface, ro rrbacv1.RoleInterface, rb rrbacv1.RoleBindingInterface) error {
	//service account
	var errgrp errgroup.Group
	errgrp.Go(func() error {
		return se.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return sa.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//role
	errgrp.Go(func() error {
		return ro.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return ro.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//rolebinding
	errgrp.Go(func() error {
		return rb.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return rb.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//service
	errgrp.Go(func() error {
		return se.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return se.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//deployment
	deleteOp := metav1.DeletePropagationBackground
	errgrp.Go(func() error {
		return dep.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{PropagationPolicy: &deleteOp})
	})
	errgrp.Go(func() error {
		return dep.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{PropagationPolicy: &deleteOp})
	})
	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func newESServiceAccount(namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedESName,
			Namespace: namespace,
		},
	}
}

func newKibanaServiceAccount(namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedKibanaName,
			Namespace: namespace,
		},
	}
}

func newESRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedESName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{"endpoints"},
				Verbs:     []string{rbacv1.VerbAll},
			},
		},
	}
}

func newKibanaRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedKibanaName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{"endpoints"},
				Verbs:     []string{rbacv1.VerbAll},
			},
		},
	}
}

func newESRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedESName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Name:     loggingconfig.EmbeddedESName,
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      loggingconfig.EmbeddedESName,
				Namespace: namespace,
			},
		},
	}
}

func newKibanaRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedKibanaName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Name:     loggingconfig.EmbeddedKibanaName,
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      loggingconfig.EmbeddedKibanaName,
				Namespace: namespace,
			},
		},
	}
}

func NewESService(namespace string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedESName,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
			},
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:       "http",
					Port:       9200,
					TargetPort: intstr.FromInt(9200),
				},
				v1.ServicePort{
					Name:       "tcp",
					Port:       9300,
					TargetPort: intstr.FromInt(9300),
				},
			},
			Selector: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
			},
		},
	}
}

func NewKibanaService(namespace string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedKibanaName,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:       "http",
					Port:       5601,
					TargetPort: intstr.FromInt(5601),
				},
			},
			Type: v1.ServiceTypeNodePort,
			Selector: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
			},
		},
	}
}

func NewESDeployment(namespace string) *v1beta2.Deployment {
	deployment := &v1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedESName,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
			},
		},
		Spec: v1beta2.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
				},
			},
			Replicas: int32Ptr(1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Labels: map[string]string{
						loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName: loggingconfig.EmbeddedESName,
					InitContainers: []v1.Container{
						{
							Name:            "init-sysctl",
							Image:           loggingconfig.BusyboxImage,
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"sysctl", "-w", "vm.max_map_count=262144"},
							SecurityContext: &v1.SecurityContext{
								Privileged: boolPtr(true),
							},
						},
					},
					Containers: []v1.Container{
						{
							Name: loggingconfig.EmbeddedESName,
							SecurityContext: &v1.SecurityContext{
								Capabilities: &v1.Capabilities{
									Add: []v1.Capability{"IPC_LOCK"},
								},
							},
							Image: loggingconfig.ESImage,
							Env: []v1.EnvVar{
								{
									Name:  "KUBERNETES_CA_CERTIFICATE_FILE",
									Value: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "CLUSTER_NAME",
									Value: "myesdb",
								},
								{
									Name:  "DISCOVERY_SERVICE",
									Value: loggingconfig.EmbeddedESName,
								},
								{
									Name:  "NODE_MASTER",
									Value: "true",
								},
								{
									Name:  "NODE_DATA",
									Value: "true",
								},
								{
									Name:  "HTTP_ENABLE",
									Value: "true",
								},
							},
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 9200,
								},
								{
									Name:          "tcp",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 9300,
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: map[v1.ResourceName]resource.Quantity{
									//CPU is always requested as an absolute quantity, never as a relative quantity; 0.1 is the same amount of CPU on a single-core, dual-core, or 48-core machine
									v1.ResourceCPU: *resource.NewMilliQuantity(int64(2000), resource.DecimalSI),
									//Limits and requests for memory are measured in bytes.
									v1.ResourceMemory: *resource.NewQuantity(int64(4*1024*1024*1024), resource.DecimalSI), // unit is byte
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									MountPath: "/data",
									Name:      "storage",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "storage",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}
	return deployment
}

func NewKibanaDeployment(namespace string) *v1beta2.Deployment {
	deployment := &v1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedKibanaName,
		},
		Spec: v1beta2.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
				},
			},
			Replicas: int32Ptr(1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Labels: map[string]string{
						loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  loggingconfig.EmbeddedKibanaName,
							Image: loggingconfig.KibanaImage,
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 5601,
								},
							},
							Env: []v1.EnvVar{
								{
									Name:  "ELASTICSEARCH_URL",
									Value: "http://" + loggingconfig.EmbeddedESName + "." + namespace + ":9200",
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}

	return deployment
}

func int32Ptr(i int32) *int32 { return &i }

func boolPtr(b bool) *bool { return &b }
