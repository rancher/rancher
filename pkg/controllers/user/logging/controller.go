package logging

import (
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
)

func Register(cluster *config.UserContext) {
	registerClusterLogging(cluster)
	registerProjectLogging(cluster)

	if err := initData(cluster.Core.Namespaces(""), cluster.Core.ConfigMaps("")); err != nil {
		logrus.Errorf("init logging data error, %v", err)
	}
}

func initData(ns rv1.NamespaceInterface, cm rv1.ConfigMapInterface) error {
	if err := utils.IniteNamespace(ns); err != nil {
		return err
	}

	return utils.InitConfigMap(cm)
}
