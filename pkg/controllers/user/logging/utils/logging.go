package utils

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func IsAllLoggingDisable(clusterLoggingLister v3.ClusterLoggingLister, projectLoggingLister v3.ProjectLoggingLister, currentCL *v3.ClusterLogging, currentPL *v3.ProjectLogging) (bool, error) {
	clusterLoggings, err := clusterLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	projectLoggings, err := projectLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	if len(clusterLoggings) == 0 && len(projectLoggings) == 0 {
		return true, nil
	}

	for _, v := range clusterLoggings {
		if currentCL != nil && v.Name == currentCL.Name {
			v = currentCL
		}
		logging, err := GetWrapConfig(v.Spec.ElasticsearchConfig, v.Spec.SplunkConfig, v.Spec.SyslogConfig, v.Spec.KafkaConfig, v.Spec.FluentForwarderConfig)
		if err != nil {
			return false, err
		}
		if logging.CurrentTarget != "" {
			return false, nil
		}
	}

	for _, v := range projectLoggings {
		if currentPL != nil && v.Name == currentPL.Name {
			v = currentPL
		}
		logging, err := GetWrapConfig(v.Spec.ElasticsearchConfig, v.Spec.SplunkConfig, v.Spec.SyslogConfig, v.Spec.KafkaConfig, v.Spec.FluentForwarderConfig)
		if err != nil {
			return false, err
		}
		if logging.CurrentTarget != "" {
			return false, nil
		}
	}
	return true, nil
}

func CleanResource(ns v1.NamespaceInterface, cl v3.ClusterLoggingLister, pl v3.ProjectLoggingLister, currentCL *v3.ClusterLogging, currentPL *v3.ProjectLogging) (bool, error) {
	allDisabled, err := IsAllLoggingDisable(cl, pl, currentCL, currentPL)
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
