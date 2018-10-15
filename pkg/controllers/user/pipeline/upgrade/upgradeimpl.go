package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	serviceName = "pipeline"
)

type pipelineService struct {
	deployments          rv1beta2.DeploymentInterface
	namespaceLister      v1.NamespaceLister
	secrets              v1.SecretInterface
	systemAccountManager *systemaccount.Manager
}

func init() {
	systemimage.RegisterSystemService(serviceName, &pipelineService{})
}

func (l *pipelineService) Init(ctx context.Context, cluster *config.UserContext) {
	l.deployments = cluster.Apps.Deployments("")
	l.namespaceLister = cluster.Core.Namespaces("").Controller().Lister()
	l.secrets = cluster.Core.Secrets("")
	l.systemAccountManager = systemaccount.NewManager(cluster.Management)
}

func (l *pipelineService) Version() (string, error) {
	d1, d2, d3 := getDeployments()
	newJekinsVersion, err := systemimage.DefaultGetVersion(d1)
	if err != nil {
		return "", err
	}

	newRegistryVersion, err := systemimage.DefaultGetVersion(d2)
	if err != nil {
		return "", err
	}

	newMinioVersion, err := systemimage.DefaultGetVersion(d3)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s", newJekinsVersion, newRegistryVersion, newMinioVersion), nil
}

func (l *pipelineService) Upgrade(currentVersion string) (newVersion string, err error) {
	var jekinsVersion, registryVersion, minioVersion, newJekinsVersion, newRegistryVersion, newMinioVersion string
	if currentVersion != "" {
		versions := strings.Split(currentVersion, "-")
		if len(versions) < 3 {
			return "", fmt.Errorf("invalid system service %s version %s", serviceName, currentVersion)
		}
		jekinsVersion = versions[0]
		registryVersion = versions[1]
		minioVersion = versions[2]
	}
	d1, d2, d3 := getDeployments()
	newJekinsVersion, err = systemimage.DefaultGetVersion(d1)
	if err != nil {
		return "", err
	}

	newRegistryVersion, err = systemimage.DefaultGetVersion(d2)
	if err != nil {
		return "", err
	}

	newMinioVersion, err = systemimage.DefaultGetVersion(d3)
	if err != nil {
		return "", err
	}

	set := labels.Set(map[string]string{utils.PipelineNamespaceLabel: "true"})
	pipelineNamespaces, err := l.namespaceLister.List("", set.AsSelector())
	if err != nil {
		return "", fmt.Errorf("list namespaces failed, %v", err)
	}
	for _, v := range pipelineNamespaces {
		ns := v.Name
		deployment := pipelineexecution.GetJenkinsDeployment(ns)
		if jekinsVersion != newJekinsVersion {
			if err = l.upgradeDeployment(deployment); err != nil {
				return "", err
			}
		}

		deployment = pipelineexecution.GetRegistryDeployment(ns)
		if registryVersion != newRegistryVersion {
			if err = l.upgradeDeployment(deployment); err != nil {
				return "", err
			}
		}

		deployment = pipelineexecution.GetMinioDeployment(ns)
		if minioVersion != newMinioVersion {
			if err = l.upgradeDeployment(deployment); err != nil {
				return "", err
			}
		}

		if err := l.ensureSecrets(v); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%s-%s-%s", newJekinsVersion, newRegistryVersion, newMinioVersion), nil
}

func (l *pipelineService) ensureSecrets(namespace *corev1.Namespace) error {
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

func (l *pipelineService) upgradeDeployment(deployment *v1beta2.Deployment) error {
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
