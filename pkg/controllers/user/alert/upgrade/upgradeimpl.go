package upgrade

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/user/alert/deploy"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/config"
	"k8s.io/api/apps/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	serviceName = "alerting"
)

type alertService struct {
	clusterName string
	deployments rv1beta2.DeploymentInterface
}

func init() {
	systemimage.RegisterSystemService(serviceName, &alertService{})
}

func (l *alertService) Init(ctx context.Context, cluster *config.UserContext) {
	l.clusterName = cluster.ClusterName
	l.deployments = cluster.Apps.Deployments("")
}

func (l *alertService) Version() (string, error) {
	deployment := deploy.GetDeployment()
	return systemimage.DefaultGetVersion(deployment)
}

func (l *alertService) Upgrade(currentVersion string) (string, error) {
	deployment := deploy.GetDeployment()
	newVersion, err := systemimage.DefaultGetVersion(deployment)
	if err != nil {
		return "", err
	}
	if currentVersion == newVersion {
		return currentVersion, nil
	}

	if err := l.upgradeDeployment(deployment); err != nil {
		return "", err
	}
	return newVersion, nil
}

func (l *alertService) upgradeDeployment(deployment *v1beta2.Deployment) error {
	if _, err := l.deployments.Update(deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("upgrade system service %s:%s failed, %v", deployment.Namespace, deployment.Name, err)
	}
	return nil
}
