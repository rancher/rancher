package app

import (
	"fmt"

	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	cattleGlobalNamespace = "cattle-global-data"
)

func addCattleGlobalNamespace(management *config.ManagementContext) error {

	lister := management.Core.Namespaces("").Controller().Lister()

	_, err := lister.Get("", cattleGlobalNamespace)
	if k8serrors.IsNotFound(err) {
		ns := &corev1.Namespace{}
		ns.Name = cattleGlobalNamespace
		if _, err := management.Core.Namespaces("").Create(ns); err != nil {
			return fmt.Errorf("Error creating %v namespace: %v", cattleGlobalNamespace, err)
		}
		logrus.Infof("Created %v namespace", cattleGlobalNamespace)
	} else if err != nil {
		return fmt.Errorf("Error creating %v namespace: %v", cattleGlobalNamespace, err)
	}

	return nil
}
