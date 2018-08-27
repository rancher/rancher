package pipelineexecution

import (
	"github.com/rancher/rancher/pkg/pipeline/utils"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	images "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/rancher/pkg/ref"
	mv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (l *Lifecycle) deploy(obj *v3.PipelineExecution) error {
	logrus.Debug("deploy pipeline workloads and services")
	nsName := utils.GetPipelineCommonName(obj)
	_, pname := ref.Parse(obj.Spec.ProjectName)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nsName,
			Labels:      labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: pname}),
			Annotations: map[string]string{nslabels.ProjectIDFieldLabel: obj.Spec.ProjectName},
		},
	}
	if _, err := l.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create ns")
	}

	secret := getSecret(nsName)
	if _, err := l.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create secret")
	}

	sa := getServiceAccount(nsName)
	if _, err := l.serviceAccounts.Create(sa); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create service account")
	}

	np := getNetworkPolicy(nsName)
	if _, err := l.networkPolicies.Create(np); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create networkpolicy")
	}

	jenkinsService := getJenkinsService(nsName)
	if _, err := l.services.Create(jenkinsService); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create jenkins service")
	}
	jenkinsDeployment := GetJenkinsDeployment(nsName)
	if _, err := l.deployments.Create(jenkinsDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create jenkins deployment")
	}

	registryService := getRegistryService(nsName)
	if _, err := l.services.Create(registryService); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create registry service")
	}
	registryDeployment := GetRegistryDeployment(nsName)
	if _, err := l.deployments.Create(registryDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create registry deployment")
	}

	minioService := getMinioService(nsName)
	if _, err := l.services.Create(minioService); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create minio service")
	}
	minioDeployment := GetMinioDeployment(nsName)
	if _, err := l.deployments.Create(minioDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create minio deployment")
	}

	return l.reconcileRb(obj)
}

func getSecret(ns string) *corev1.Secret {
	token, err := randomtoken.Generate()
	if err != nil {
		logrus.Warningf("warning generate random token got - %v, use default instead", err)
		token = utils.PipelineSecretDefaultToken
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.PipelineSecretName,
		},
		Data: map[string][]byte{
			utils.PipelineSecretTokenKey: []byte(token),
			utils.PipelineSecretUserKey:  []byte(utils.PipelineSecretDefaultUser),
		},
	}
}

func getServiceAccount(ns string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.JenkinsName,
		},
	}
}

func getRoleBindings(rbNs string, commonName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      commonName,
			Namespace: rbNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleAdmin,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: commonName,
			Name:      utils.JenkinsName,
		}},
	}
}

func getClusterRoleBindings(ns string, roleName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns + "-" + roleName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: ns,
			Name:      utils.JenkinsName,
		}},
	}
}

func getJenkinsService(ns string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.JenkinsName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				utils.LabelKeyApp:     utils.JenkinsName,
				utils.LabelKeyJenkins: utils.JenkinsMaster,
			},
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: utils.JenkinsPort,
				},
				{
					Name: "agent",
					Port: utils.JenkinsJNLPPort,
				},
			},
		},
	}
}

func GetJenkinsDeployment(ns string) *appsv1beta2.Deployment {
	replicas := int32(1)
	return &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.JenkinsName,
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{utils.LabelKeyApp: utils.JenkinsName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						utils.LabelKeyApp:     utils.JenkinsName,
						utils.LabelKeyJenkins: utils.JenkinsMaster,
					},
					Name: utils.JenkinsName,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: utils.JenkinsName,
					Containers: []corev1.Container{
						{
							Name:  utils.JenkinsName,
							Image: images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.Jenkins),
							Env: []corev1.EnvVar{
								{
									Name: "ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: utils.PipelineSecretName,
											},
											Key: utils.PipelineSecretTokenKey,
										}},
								}, {
									Name: "ADMIN_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: utils.PipelineSecretName,
											},
											Key: utils.PipelineSecretUserKey,
										}},
								}, {
									Name:  "JAVA_OPTS",
									Value: "-Xmx300m -Dhudson.slaves.NodeProvisioner.initialDelay=0 -Dhudson.slaves.NodeProvisioner.MARGIN=50 -Dhudson.slaves.NodeProvisioner.MARGIN0=0.85 -Dhudson.model.LoadStatistics.clock=2000 -Dhudson.slaves.NodeProvisioner.recurrencePeriod=2000",
								}, {
									Name:  "NAMESPACE",
									Value: ns,
								}, {
									Name: "JENKINS_POD_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: utils.JenkinsPort,
								},
								{
									Name:          "agent",
									ContainerPort: utils.JenkinsJNLPPort,
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/login",
										Port: intstr.FromInt(utils.JenkinsPort),
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

func getNetworkPolicy(ns string) *v1.NetworkPolicy {
	return &v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.NetWorkPolicyName,
		},
		Spec: v1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      utils.LabelKeyApp,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{utils.JenkinsName, utils.MinioName},
					},
				},
			},
			Ingress: []v1.NetworkPolicyIngressRule{{}},
		},
	}
}

func getRegistryService(ns string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.RegistryName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				utils.LabelKeyApp: utils.RegistryName,
			},
			Ports: []corev1.ServicePort{
				{
					Name: utils.RegistryName,
					Port: utils.RegistryPort,
				},
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
}

func GetRegistryDeployment(ns string) *appsv1beta2.Deployment {
	replicas := int32(1)
	return &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.RegistryName,
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{utils.LabelKeyApp: utils.RegistryName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{utils.LabelKeyApp: utils.RegistryName},
					Name:   utils.RegistryName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            utils.RegistryName,
							Image:           images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.Registry),
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									Name:          utils.RegistryName,
									ContainerPort: utils.RegistryPort,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getMinioService(ns string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.MinioName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				utils.LabelKeyApp: utils.MinioName,
			},
			Ports: []corev1.ServicePort{
				{
					Name: utils.MinioName,
					Port: utils.MinioPort,
				},
			},
		},
	}
}

func GetMinioDeployment(ns string) *appsv1beta2.Deployment {
	replicas := int32(1)
	return &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.MinioName,
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{utils.LabelKeyApp: utils.MinioName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{utils.LabelKeyApp: utils.MinioName},
					Name:   utils.MinioName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            utils.MinioName,
							Image:           images.Resolve(mv3.ToolsSystemImages.PipelineSystemImages.Minio),
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"server", "/data"},
							Env: []corev1.EnvVar{
								{
									Name: "MINIO_SECRET_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: utils.PipelineSecretName,
											},
											Key: utils.PipelineSecretTokenKey,
										}},
								}, {
									Name: "MINIO_ACCESS_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: utils.PipelineSecretName,
											},
											Key: utils.PipelineSecretUserKey,
										}},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          utils.MinioName,
									ContainerPort: utils.MinioPort,
								},
							},
						},
					},
				},
			},
		},
	}
}
