package utils

import (
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	rv1 "github.com/rancher/types/apis/core/v1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IniteNamespace(ns rv1.NamespaceInterface) error {
	if _, err := ns.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.LoggingNamespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		initNamespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: loggingconfig.LoggingNamespace,
			},
		}

		if _, err := ns.Create(&initNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}
