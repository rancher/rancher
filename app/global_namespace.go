package app

import (
	"fmt"

	"github.com/rancher/rancher/pkg/settings"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func addCattleGlobalNamespace(management *config.ManagementContext) error {

	lister := management.Core.Namespaces("").Controller().Lister()

	_, err := lister.Get("", namespace.GlobalNamespace)
	if k8serrors.IsNotFound(err) {
		ns := &corev1.Namespace{}
		ns.Name = namespace.GlobalNamespace
		if _, err := management.Core.Namespaces("").Create(ns); err != nil {
			return fmt.Errorf("Error creating %v namespace: %v", namespace.GlobalNamespace, err)
		}
		logrus.Infof("Created %v namespace", namespace.GlobalNamespace)
	} else if err != nil {
		return fmt.Errorf("Error creating %v namespace: %v", namespace.GlobalNamespace, err)
	}

	logrus.Debugf("calling sync for driver metadata")
	management.Management.Settings("").Controller().Enqueue("", settings.RkeMetadataConfig.Name)

	return nil
}
