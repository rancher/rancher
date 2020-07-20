package upgrade

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/controllers/managementuser/pipeline/controller/pipelineexecution"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	images "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	ServiceName = "pipeline"
)

type PipelineService struct {
	deployments          appsv1.DeploymentInterface
	deploymentLister     appsv1.DeploymentLister
	namespaceLister      v1.NamespaceLister
	secrets              v1.SecretInterface
	systemAccountManager *systemaccount.Manager
}

func NewService() *PipelineService {
	return &PipelineService{}
}

func (l *PipelineService) Init(cluster *config.UserContext) {
	l.deployments = cluster.Apps.Deployments("")
	l.deploymentLister = cluster.Apps.Deployments("").Controller().Lister()
	l.namespaceLister = cluster.Core.Namespaces("").Controller().Lister()
	l.secrets = cluster.Core.Secrets("")
	l.systemAccountManager = systemaccount.NewManager(cluster.Management)
}

func (l *PipelineService) Version() (string, error) {
	raw := fmt.Sprintf("%s-%s-%s",
		v32.ToolsSystemImages.PipelineSystemImages.Jenkins,
		v32.ToolsSystemImages.PipelineSystemImages.Registry,
		v32.ToolsSystemImages.PipelineSystemImages.Minio)
	return getDefaultVersion(raw)
}

func (l *PipelineService) Upgrade(currentVersion string) (newVersion string, err error) {
	set := labels.Set(map[string]string{utils.PipelineNamespaceLabel: "true"})
	pipelineNamespaces, err := l.namespaceLister.List("", set.AsSelector())
	if err != nil {
		return "", fmt.Errorf("list namespaces failed, %v", err)
	}
	for _, v := range pipelineNamespaces {
		if err := l.ensureSecrets(v); err != nil {
			return "", err
		}
		if err := l.upgradeComponents(v.Name); err != nil {
			return "", err
		}
	}

	return l.Version()
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

func (l *PipelineService) upgradeComponents(ns string) error {
	jenkinsDeployment := pipelineexecution.GetJenkinsDeployment(ns)
	if _, err := l.deployments.Update(jenkinsDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("upgrade system service %s:%s failed, %v", jenkinsDeployment.Namespace, jenkinsDeployment.Name, err)
	}

	//Only update image for Registry and Minio to preserve user customized configurations such as volumes
	registryDeployment, err := l.deploymentLister.Get(ns, utils.RegistryName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	toUpdateRegistry := registryDeployment.DeepCopy()
	toUpdateRegistry.Spec.Template.Spec.Containers[0].Image = images.Resolve(v32.ToolsSystemImages.PipelineSystemImages.Registry)
	if _, err := l.deployments.Update(toUpdateRegistry); err != nil {
		return err
	}
	minioDeployment, err := l.deploymentLister.Get(ns, utils.MinioName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	toUpdateMinio := minioDeployment.DeepCopy()
	toUpdateMinio.Spec.Template.Spec.Containers[0].Image = images.Resolve(v32.ToolsSystemImages.PipelineSystemImages.Minio)
	if _, err := l.deployments.Update(toUpdateMinio); err != nil {
		return err
	}

	return nil
}

func getDefaultVersion(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal obj failed when get system image version: %v", err)
	}

	return fmt.Sprintf("%x", sha1.Sum(b))[:7], nil
}
