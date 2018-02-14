package deploy

import (
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	yaml "gopkg.in/yaml.v2"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EmailTmlp = `{{ define "email.text" }}{{ (index .Alerts 0).Labels.text}} {{ end}}"`
)

func NewDeployer(cluster *config.UserContext, manager *manager.Manager) *Deployer {
	return &Deployer{
		nsClient:     cluster.Core.Namespaces(""),
		appsClient:   cluster.Apps.Deployments(""),
		secretClient: cluster.Core.Secrets(""),
		svcClient:    cluster.Core.Services(""),
		alertManager: manager,
	}
}

type Deployer struct {
	nsClient     v1.NamespaceInterface
	appsClient   v1beta2.DeploymentInterface
	secretClient v1.SecretInterface
	svcClient    v1.ServiceInterface
	alertManager *manager.Manager
}

func (d *Deployer) ProjectSync(key string, alert *v3.ProjectAlert) error {
	return d.deploy()
}

func (d *Deployer) ClusterSync(key string, alert *v3.ClusterAlert) error {
	return d.deploy()
}

func (d *Deployer) deploy() error {

	//TODO: cleanup resources while there is not any alert configured
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cattle-alerting",
		},
	}
	if _, err := d.nsClient.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating ns")
	}

	secret := d.getSecret()
	if _, err := d.secretClient.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating secret")
	}

	deployment := getDeployment()
	if _, err := d.appsClient.Create(deployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating deployment")
	}

	service := getService()
	if _, err := d.svcClient.Create(service); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating service")
	}

	return nil
}

func (d *Deployer) getSecret() *corev1.Secret {
	cfg := d.alertManager.GetDefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cattle-alerting",
			Name:      "alertmanager",
		},
		Data: map[string][]byte{
			"config.yml": data,
			"email.tmpl": []byte(EmailTmlp),
		},
	}
}

func getService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cattle-alerting",
			Name:      "alertmanager",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "alertmanager",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "alertmanager",
					Port: 9093,
				},
			},
		},
	}
}

func getDeployment() *appsv1beta2.Deployment {
	replicas := int32(1)
	return &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cattle-alerting",
			Name:      "alertmanager",
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "alertmanager"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "alertmanager"},
					Name:   "alertmanager",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "alertmanager",
							Image: "prom/alertmanager:v0.11.0",
							Args:  []string{"-config.file=/etc/alertmanager/config.yml", "-storage.path=/alertmanager"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "alertmanager",
									ContainerPort: 9093,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "alertmanager",
									MountPath: "/alertmanager",
								},
								{
									Name:      "config-volume",
									MountPath: "/etc/alertmanager",
								},
							},
						},
						{
							Name:    "alertmanager-helper",
							Image:   "rancher/alertmanager-helper:v0.0.2",
							Command: []string{"alertmanager-helper"},
							Args:    []string{"--watched-file-list", "/etc/alertmanager"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/alertmanager",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "alertmanager",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "alertmanager",
								},
							},
						},
					},
				},
			},
		},
	}
}
