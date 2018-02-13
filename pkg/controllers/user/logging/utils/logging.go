package utils

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func IsAllLoggingDisable(clusterLoggingLister v3.ClusterLoggingLister, projectLoggingLister v3.ProjectLoggingLister) (bool, error) {
	clusterLoggings, err := clusterLoggingLister.List(loggingconfig.LoggingNamespace, labels.NewSelector())
	if err != nil {
		return false, err
	}

	projectLoggings, err := projectLoggingLister.List(loggingconfig.LoggingNamespace, labels.NewSelector())
	if err != nil {
		return false, err
	}
	return len(clusterLoggings) == 0 && len(projectLoggings) == 0, nil

}
