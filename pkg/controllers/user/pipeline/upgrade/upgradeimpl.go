package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/user/pipeline/controller/pipelineexecution"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	"k8s.io/api/apps/v1beta2"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	serviceName = "pipeline"
)

type pipelineService struct {
	deployments   rv1beta2.DeploymentInterface
	projectLister v3.ProjectLister
}

func init() {
	systemimage.RegisterSystemService(serviceName, &pipelineService{})
}

func (l *pipelineService) Init(ctx context.Context, cluster *config.UserContext) {
	l.deployments = cluster.Apps.Deployments("")
	l.projectLister = cluster.Management.Management.Projects("").Controller().Lister()
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

	projects, err := l.projectLister.List("", labels.NewSelector())
	if err != nil {
		return "", fmt.Errorf("list project failed, %v", err)
	}
	for _, v := range projects {
		ns := v.Name + utils.PipelineNamespaceSuffix
		deployment := pipelineexecution.GetJenkinsDeployment(ns)
		if jekinsVersion != newJekinsVersion {
			if err = l.upgradeDeployment(deployment); err != nil {
				return "", err
			}
		}

		deployment = pipelineexecution.GetRegistryDeployment(ns)
		if registryVersion != newJekinsVersion {
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
	}

	return fmt.Sprintf("%s-%s-%s", newJekinsVersion, newRegistryVersion, newMinioVersion), nil
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
