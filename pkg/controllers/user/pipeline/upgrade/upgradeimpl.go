package upgrade

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	ServiceName = "pipeline"
)

type PipelineService struct {
	deployments          rv1beta2.DeploymentInterface
	namespaceLister      v1.NamespaceLister
	secrets              v1.SecretInterface
	systemAccountManager *systemaccount.Manager
}

func NewService() *PipelineService {
	return &PipelineService{}
}

func (l *PipelineService) Init(cluster *config.UserContext) {
	l.deployments = cluster.Apps.Deployments("")
	l.namespaceLister = cluster.Core.Namespaces("").Controller().Lister()
	l.secrets = cluster.Core.Secrets("")
	l.systemAccountManager = systemaccount.NewManager(cluster.Management)
}

func (l *PipelineService) Version() (string, error) {
	d1, d2, d3 := getDeployments()
	newJekinsVersion, err := getDefaultVersion(d1)
	if err != nil {
		return "", err
	}

	newRegistryVersion, err := getDefaultVersion(d2)
	if err != nil {
		return "", err
	}

	newMinioVersion, err := getDefaultVersion(d3)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s", newJekinsVersion, newRegistryVersion, newMinioVersion), nil
}

func (l *PipelineService) Upgrade(currentVersion string) (newVersion string, err error) {
	var newJekinsVersion, newRegistryVersion, newMinioVersion string

	d1, d2, d3 := getDeployments()
	newJekinsVersion, err = getDefaultVersion(d1)
	if err != nil {
		return "", err
	}

	newRegistryVersion, err = getDefaultVersion(d2)
	if err != nil {
		return "", err
	}

	newMinioVersion, err = getDefaultVersion(d3)
	if err != nil {
		return "", err
	}

	set := labels.Set(map[string]string{utils.PipelineNamespaceLabel: "true"})
	pipelineNamespaces, err := l.namespaceLister.List("", set.AsSelector())
	if err != nil {
		return "", fmt.Errorf("list namespaces failed, %v", err)
	}
	for _, v := range pipelineNamespaces {
		if err := l.ensureSecrets(v); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%s-%s-%s", newJekinsVersion, newRegistryVersion, newMinioVersion), nil
}

func (l *PipelineService) ensureSecrets(namespace *corev1.Namespace) error {
	projectName := namespace.Annotations["field.cattle.io/projectId"]
	_, projectID := ref.Parse(projectName)
	ns := namespace.Name
	apikey, err := l.systemAccountManager.GetOrCreateProjectSystemToken(projectID)
	if err != nil {
		return err
	}
	secret := pipelineexecution.GetAPIKeySecret(ns, apikey)
	if _, err := l.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (l *PipelineService) upgradeDeployment(deployment *v1beta2.Deployment) error {
	if _, err := l.deployments.Update(deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("upgrade system service %s:%s failed, %v", deployment.Namespace, deployment.Name, err)
	}
	return nil
}

func getDeployments() (d1, d2, d3 *v1beta2.Deployment) {
	d1 = pipelineexecution.GetJenkinsDeployment("")
	d2 = pipelineexecution.GetRegistryDeployment("")
	d3 = pipelineexecution.GetMinioDeployment("")
	return d1, d2, d3
}

func getDefaultVersion(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal obj failed when get system image version: %v", err)
	}

	return fmt.Sprintf("%x", sha1.Sum(b))[:7], nil
}
