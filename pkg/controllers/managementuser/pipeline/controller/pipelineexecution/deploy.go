package pipelineexecution

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	images "github.com/rancher/rancher/pkg/image"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const projectIDFieldLabel = "field.cattle.io/projectId"
const defaultPortRange = "34000-35000"

func (l *Lifecycle) deploy(projectName string) error {

	clusterID, projectID := ref.Parse(projectName)
	ns := getPipelineNamespace(clusterID, projectID)
	if _, err := l.namespaceLister.Get("", ns.Name); err == nil {
		return l.reconcileRb(projectName)
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	logrus.Debug("deploy pipeline workloads and services")
	if _, err := l.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the pipeline namespace")
	}
	if err := l.waitResourceQuotaInitCondition(ns.Name); err != nil {
		return err
	}
	token, err := randomtoken.Generate()
	if err != nil {
		logrus.Warningf("warning generate random token got - %v, use default instead", err)
		token = utils.PipelineSecretDefaultToken
	}

	nsName := utils.GetPipelineCommonName(projectName)
	ns = getCommonPipelineNamespace()
	if _, err := l.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the cattle-pipeline namespace")
	}
	secret := getPipelineSecret(nsName, token)
	for i := 0; i < 3; i++ {
		// If project resource quota is enabled, It won't succeed until the resource quota in namespace is synced.
		// Do retries for this case.
		if _, err = l.secrets.Create(secret); err == nil || apierrors.IsAlreadyExists(err) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating a pipeline secret")
	}

	apikey, err := l.systemAccountManager.GetOrCreateProjectSystemToken(projectID)
	if err != nil {
		return err
	}
	secret = GetAPIKeySecret(nsName, apikey)
	if _, err := l.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating a pipeline secret")
	}

	if err := l.reconcileRegistryCASecret(clusterID); err != nil {
		return err
	}
	if err := l.reconcileRegistryCrtSecret(clusterID, projectID); err != nil {
		return err
	}

	sa := getServiceAccount(nsName)
	if _, err := l.serviceAccounts.Create(sa); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating a pipeline service account")
	}
	np := getNetworkPolicy(nsName)
	if _, err := l.networkPolicies.Create(np); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create a pipeline networkpolicy")
	}
	jenkinsService := getJenkinsService(nsName)
	if _, err := l.services.Create(jenkinsService); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the jenkins service")
	}
	jenkinsDeployment := GetJenkinsDeployment(nsName)
	if _, err := l.deployments.Create(jenkinsDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the jenkins deployment")
	}
	registryService := getRegistryService(nsName)
	if _, err := l.services.Create(registryService); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the registry service")
	}
	registryDeployment := GetRegistryDeployment(nsName)
	if _, err := l.deployments.Create(registryDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the registry deployment")
	}
	minioService := getMinioService(nsName)
	if _, err := l.services.Create(minioService); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the minio service")
	}
	minioDeployment := GetMinioDeployment(nsName)
	if _, err := l.deployments.Create(minioDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the minio deployment")
	}

	if err := l.reconcileProxyConfigMap(projectID); err != nil {
		return err
	}
	//docker credential for local registry
	if err := l.reconcileRegistryCredential(projectName, token); err != nil {
		return err
	}
	nginxDaemonset := GetProxyDaemonset()
	if _, err := l.daemonsets.Create(nginxDaemonset); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error creating the nginx proxy")
	}

	return l.reconcileRb(projectName)
}

func (l *Lifecycle) waitResourceQuotaInitCondition(namespace string) error {
	tries := 0
	for tries <= 3 {
		ns, err := l.namespaces.Get(namespace, metav1.GetOptions{})
		if err != nil {
			return err
		}

		invalid, err := namespaceutil.IsNamespaceConditionSet(ns, resourcequota.ResourceQuotaValidatedCondition, false)
		if err != nil {
			return err
		}
		if invalid {
			return fmt.Errorf("available resource quota in this project does not fit for the dedicated pipeline namespace, please make sure there is vacancy for the default namespace quota")
		}

		set, err := namespaceutil.IsNamespaceConditionSet(ns, resourcequota.ResourceQuotaInitCondition, true)
		if err != nil {
			return err
		}
		if set {
			break
		}
		time.Sleep(2 * time.Second)
		tries++
	}
	return nil
}

func getCommonPipelineNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.PipelineNamespace,
		},
	}
}

func getPipelineNamespace(clusterID, projectID string) *corev1.Namespace {
	ns := projectID + utils.PipelineNamespaceSuffix
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
			Labels: labels.Set(map[string]string{
				nslabels.ProjectIDFieldLabel: projectID,
				utils.PipelineNamespaceLabel: "true",
			}),
			Annotations: map[string]string{nslabels.ProjectIDFieldLabel: ref.FromStrings(clusterID, projectID)},
		},
	}
}

func getPipelineSecret(ns string, token string) *corev1.Secret {
	hashed, err := utils.BCryptHash(token)
	if err != nil {
		logrus.Warningf("warning hash registry token got - %v", err)
	}
	registryToken := fmt.Sprintf("%s:%s", utils.PipelineSecretDefaultUser, hashed)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.PipelineSecretName,
		},
		Data: map[string][]byte{
			utils.PipelineSecretTokenKey:         []byte(token),
			utils.PipelineSecretUserKey:          []byte(utils.PipelineSecretDefaultUser),
			utils.PipelineSecretRegistryTokenKey: []byte(registryToken),
		},
	}
}

func GetAPIKeySecret(ns string, key string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.PipelineAPIKeySecretName,
		},
		Data: map[string][]byte{
			utils.PipelineSecretAPITokenKey: []byte(key),
		},
	}
}

func (l *Lifecycle) reconcileRegistryCASecret(clusterID string) error {
	cn := "docker-registry-ca"
	CACrt, CAKey, err := pki.GenerateCACertAndKey(cn, nil)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterID,
			Name:      utils.RegistryCACrtSecretName,
		},
		Data: map[string][]byte{
			utils.RegistryCACrt: cert.EncodeCertPEM(CACrt),
			utils.RegistryCAKey: cert.EncodePrivateKeyPEM(CAKey),
		},
	}
	if _, err := l.managementSecrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create registry ca secret")
	}
	return nil
}

func (l *Lifecycle) reconcileRegistryCrtSecret(clusterID, projectID string) error {
	ns := projectID + utils.PipelineNamespaceSuffix
	// generate domain cert & key if they do not exist
	_, err := l.secrets.GetNamespaced(ns, utils.RegistryCrtSecretName, metav1.GetOptions{})
	if err == nil || !apierrors.IsNotFound(err) {
		return err
	}

	// ca cert for proxy
	caSecret, err := l.managementSecrets.GetNamespaced(clusterID, utils.RegistryCACrtSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	crtRaw := caSecret.Data[utils.RegistryCACrt]
	keyRaw := caSecret.Data[utils.RegistryCAKey]

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      utils.RegistryCACrtSecretName,
		},
		Data: map[string][]byte{
			utils.RegistryCACrt: crtRaw,
		},
	}
	if _, err := l.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	caCrt, err := cert.ParseCertsPEM(crtRaw)
	if err != nil || len(caCrt) < 1 {
		return errors.Wrap(err, "invalid pem format")
	}
	caKey, err := cert.ParsePrivateKeyPEM(keyRaw)

	if _, ok := caKey.(*rsa.PrivateKey); !ok || err != nil {
		return errors.Wrap(err, "invalid pem format")
	}
	cfg := cert.Config{
		CommonName:   utils.RegistryName,
		Organization: []string{},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: cert.AltNames{
			DNSNames: []string{
				utils.RegistryName,
				fmt.Sprintf("%s.%s", utils.RegistryName, ns),
				fmt.Sprintf("%s.%s.svc", utils.RegistryName, ns),
			},
		},
	}
	key, err := cert.NewPrivateKey()
	if err != nil {
		return err
	}
	duration := getSigningDuration(l.pipelineSettingLister, projectID)
	crt, err := newSignedCertWithDuration(cfg, duration, key, caCrt[0], caKey.(*rsa.PrivateKey))
	if err != nil {
		return err
	}

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.RegistryCrtSecretName,
		},
		Data: map[string][]byte{
			utils.RegistryCrt:   cert.EncodeCertPEM(crt),
			utils.RegistryKey:   cert.EncodePrivateKeyPEM(key),
			utils.RegistryCACrt: crtRaw,
		},
	}
	if _, err := l.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create secret")
	}
	return nil
}

func getRegistryCredential(projectID string, token string, hostname string) (*corev1.Secret, error) {
	_, ns := ref.Parse(projectID)
	config := credentialprovider.DockerConfigJson{
		Auths: credentialprovider.DockerConfig{
			hostname: credentialprovider.DockerConfigEntry{
				Username: utils.PipelineSecretDefaultUser,
				Password: token,
				Email:    "",
			},
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.DockerCredentialName,
			Namespace: ns,
			Annotations: map[string]string{
				projectIDFieldLabel: projectID,
			},
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: configJSON,
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}, nil
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

func GetJenkinsDeployment(ns string) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.JenkinsName,
		},
		Spec: appsv1.DeploymentSpec{
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
							Image: images.Resolve(v33.ToolsSystemImages.PipelineSystemImages.Jenkins),
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
									Value: "-Xmx300m -Dhudson.slaves.NodeProvisioner.initialDelay=0 -Dhudson.slaves.NodeProvisioner.MARGIN=50 -Dhudson.slaves.NodeProvisioner.MARGIN0=0.85 -Dhudson.model.LoadStatistics.clock=2000 -Dhudson.slaves.NodeProvisioner.recurrencePeriod=2000 -Dhudson.model.UpdateCenter.never=true",
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
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(1024e6, resource.BinarySI),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(300e6, resource.BinarySI),
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
		},
	}
}

func GetRegistryDeployment(ns string) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.RegistryName,
		},
		Spec: appsv1.DeploymentSpec{
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
							Image:           images.Resolve(v33.ToolsSystemImages.PipelineSystemImages.Registry),
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									Name:          utils.RegistryName,
									ContainerPort: utils.RegistryPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "REGISTRY_HTTP_ADDR",
									Value: "0.0.0.0:443",
								},
								{
									Name:  "REGISTRY_HTTP_TLS_CERTIFICATE",
									Value: utils.RegistryCrtPath + utils.RegistryCrt,
								},
								{
									Name:  "REGISTRY_HTTP_TLS_KEY",
									Value: utils.RegistryCrtPath + utils.RegistryKey,
								},
								{
									Name:  "REGISTRY_AUTH",
									Value: "htpasswd",
								},
								{
									Name:  "REGISTRY_AUTH_HTPASSWD_REALM",
									Value: "Registry Realm",
								},
								{
									Name:  "REGISTRY_AUTH_HTPASSWD_PATH",
									Value: utils.RegistryAuthPath + utils.PipelineSecretRegistryTokenKey,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(200e6, resource.BinarySI),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(100e6, resource.BinarySI),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      utils.RegistryCrtVolumeName,
									MountPath: utils.RegistryCrtPath,
									ReadOnly:  true,
								},
								{
									Name:      utils.RegistryAuthVolumeName,
									MountPath: utils.RegistryAuthPath,
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: utils.RegistryCrtVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: utils.RegistryCrtSecretName,
								},
							},
						},
						{
							Name: utils.RegistryAuthVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: utils.PipelineSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  utils.PipelineSecretRegistryTokenKey,
											Path: utils.PipelineSecretRegistryTokenKey,
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

func (l *Lifecycle) reconcileProxyConfigMap(projectID string) error {
	exist := true
	cm, err := l.configMapLister.Get(utils.PipelineNamespace, utils.ProxyConfigMapName)
	if apierrors.IsNotFound(err) {
		exist = false
	} else if err != nil {
		return err
	}

	if !exist {
		port, err := l.getAvailablePort()
		if err != nil {
			return err
		}
		toCreate := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: utils.PipelineNamespace,
				Name:      utils.ProxyConfigMapName,
			},
			Data: map[string]string{},
		}
		if err := utils.SetRegistryPortMapping(toCreate, map[string]string{projectID: port}); err != nil {
			return err
		}
		_, err = l.configMaps.Create(toCreate)
		return err
	}

	portMap, err := utils.GetRegistryPortMapping(cm)
	if err != nil {
		return err
	}
	if _, ok := portMap[projectID]; !ok {
		port, err := l.getAvailablePort()
		if err != nil {
			return err
		}
		toUpdate := cm.DeepCopy()
		portMap, err = utils.GetRegistryPortMapping(toUpdate)
		if err != nil {
			return err
		}

		portMap[projectID] = port
		if err := utils.SetRegistryPortMapping(toUpdate, portMap); err != nil {
			return err
		}
		_, err = l.configMaps.Update(toUpdate)
		return err
	}

	return nil
}

func (l *Lifecycle) getAvailablePort() (string, error) {
	portRange := &net.PortRange{}
	portRange.Set(defaultPortRange)
	rand.Seed(time.Now().UnixNano())
	rd := rand.Intn(portRange.Size)

	cm, err := l.configMapLister.Get(utils.PipelineNamespace, utils.ProxyConfigMapName)
	if apierrors.IsNotFound(err) {
		return strconv.Itoa(rd + portRange.Base), nil
	} else if err != nil {
		return "", err
	}

	portMap, err := utils.GetRegistryPortMapping(cm)
	if err != nil {
		return "", err
	}
	usedPorts := map[string]bool{}
	for _, p := range portMap {
		usedPorts[p] = true
	}

	for i := 0; i < portRange.Size; i++ {
		port := strconv.Itoa((rd+i)%portRange.Size + portRange.Base)
		if !usedPorts[port] {
			return port, nil
		}
	}

	return "", errors.New("No available port")

}

func GetProxyDaemonset() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      utils.RegistryProxyName,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{utils.LabelKeyApp: utils.RegistryProxyName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{utils.LabelKeyApp: utils.RegistryProxyName},
					Name:   utils.RegistryProxyName,
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
					Containers: []corev1.Container{
						{
							Name:    utils.RegistryProxyName,
							Image:   images.Resolve(v33.ToolsSystemImages.PipelineSystemImages.RegistryProxy),
							Command: []string{"nginx-proxy"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      utils.RegistryPortMappingKey,
									MountPath: utils.RegistryMappingPath,
								},
								{
									Name:      utils.RegistryCrtVolumeName,
									MountPath: utils.RegistryCrtPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: utils.RegistryPortMappingKey,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: utils.ProxyConfigMapName},
									Items: []corev1.KeyToPath{
										{Key: utils.RegistryPortMappingFile,
											Path: utils.RegistryPortMappingFile,
										},
									},
								},
							},
						},
						{
							Name: utils.RegistryCrtVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: utils.RegistryCACrtSecretName,
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

func GetMinioDeployment(ns string) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      utils.MinioName,
		},
		Spec: appsv1.DeploymentSpec{
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
							Image:           images.Resolve(v33.ToolsSystemImages.PipelineSystemImages.Minio),
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
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(500e6, resource.BinarySI),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(100e6, resource.BinarySI),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (l *Lifecycle) reconcileRegistryCredential(projectName, token string) error {
	cm, err := l.configMaps.GetNamespaced(utils.PipelineNamespace, utils.ProxyConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	portMap, err := utils.GetRegistryPortMapping(cm)
	if err != nil {
		return err
	}
	_, projectID := ref.Parse(projectName)
	port, ok := portMap[projectID]
	if !ok || port == "" {
		return errors.New("Found no port for local registry")
	}
	regHostname := "127.0.0.1:" + port
	dockerCredential, err := getRegistryCredential(projectName, token, regHostname)
	if err != nil {
		return err
	}
	if _, err := l.managementSecrets.Create(dockerCredential); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create credential for local registry")
	}
	return nil
}
