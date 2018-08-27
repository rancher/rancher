package upgrade

import (
	"context"
	"fmt"
	"strings"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/controllers/user/systemimage"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	"k8s.io/api/apps/v1beta2"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	serviceName = "logging"
)

type loggingService struct {
	clusterName          string
	clusterLister        v3.ClusterLister
	clusterLoggingLister v3.ClusterLoggingLister
	daemonsets           rv1beta2.DaemonSetInterface
	projectLoggingLister v3.ProjectLoggingLister
}

func init() {
	systemimage.RegisterSystemService(serviceName, &loggingService{})
}

func (l *loggingService) Init(ctx context.Context, cluster *config.UserContext) {
	l.clusterName = cluster.ClusterName
	l.clusterLister = cluster.Management.Management.Clusters("").Controller().Lister()
	l.clusterLoggingLister = cluster.Management.Management.ClusterLoggings(cluster.ClusterName).Controller().Lister()
	l.daemonsets = cluster.Apps.DaemonSets(loggingconfig.LoggingNamespace)
	l.projectLoggingLister = cluster.Management.Management.ProjectLoggings("").Controller().Lister()
}

func (l *loggingService) Version() (string, error) {
	_, _, fluentdVersion, logAggregatorVersion, err := getDaemonset("", "")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s", fluentdVersion, logAggregatorVersion), nil
}

func (l *loggingService) Upgrade(currentVersion string) (string, error) {
	var fluentdVersion, logAggregatorVersion, newFluentdVersion, newLogAggregatorVersion string
	if currentVersion != "" {
		versions := strings.Split(currentVersion, "-")
		if len(versions) < 2 {
			return currentVersion, fmt.Errorf("invalid %s system service version %s", serviceName, currentVersion)
		}
		fluentdVersion = versions[0]
		logAggregatorVersion = versions[1]
	}

	cluster, err := l.clusterLister.Get("", l.clusterName)
	if err != nil {
		return "", fmt.Errorf("get cluster %s failed, %v", l.clusterName, err)
	}

	fluentd, logAggregator, newFluentdVersion, newLogAggregatorVersion, err := getDaemonset(cluster.Spec.DockerRootDir, cluster.Status.Driver)
	if err != nil {
		return "", err
	}

	if fluentdVersion != newFluentdVersion {
		err = l.upgradeDaemonset(fluentd)
		if err != nil {
			return "", err
		}
	}

	if logAggregatorVersion != newLogAggregatorVersion {
		if err = l.upgradeDaemonset(logAggregator); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%s-%s", newFluentdVersion, newLogAggregatorVersion), nil
}

func (l *loggingService) upgradeDaemonset(daemonset *v1beta2.DaemonSet) error {
	if _, err := l.daemonsets.Update(daemonset); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("upgrade system service %s:%s failed, %v", daemonset.Namespace, daemonset.Name, err)
	}
	return nil
}

func getDaemonset(dockerRootDir, driver string) (fluentd, logAggregator *v1beta2.DaemonSet, newFluentdVersion, newLogAggregatorVersion string, err error) {
	fluentd = utils.NewFluentdDaemonset(loggingconfig.FluentdName, loggingconfig.LoggingNamespace, dockerRootDir)
	logAggregator = utils.NewLogAggregatorDaemonset(loggingconfig.LogAggregatorName, loggingconfig.LoggingNamespace, driver)

	newFluentdVersion, err = systemimage.DefaultGetVersion(fluentd)
	if err != nil {
		return
	}

	newLogAggregatorVersion, err = systemimage.DefaultGetVersion(logAggregator)
	if err != nil {
		return
	}
	return
}
