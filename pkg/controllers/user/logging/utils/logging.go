package utils

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func CleanResource(ns v1.NamespaceInterface, cl v3.ClusterLoggingLister, pl v3.ProjectLoggingLister) (bool, error) {
	allDisabled, err := IsAllLoggingDisable(cl, pl)
	if err != nil {
		return allDisabled, err
	}

	var zero int64
	foreground := metav1.DeletePropagationForeground
	if allDisabled {
		if err = ns.Delete(loggingconfig.LoggingNamespace, &metav1.DeleteOptions{GracePeriodSeconds: &zero, PropagationPolicy: &foreground}); err != nil && !apierrors.IsNotFound(err) {
			return allDisabled, err
		}
	}
	return allDisabled, nil
}
