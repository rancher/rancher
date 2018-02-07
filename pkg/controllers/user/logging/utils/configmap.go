package utils

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	rv1 "github.com/rancher/types/apis/core/v1"
	"golang.org/x/sync/errgroup"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func InitConfigMap(cm rv1.ConfigMapInterface) error {
	if _, err := cm.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.ClusterLoggingName); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if _, err := cm.Create(newClusterLoggingConfigMap()); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	if _, err := cm.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.ProjectLoggingName); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if _, err := cm.Create(newProjectLoggingConfigMap()); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func UpdateConfigMap(configPath, loggingName, level string, configmaps rv1.ConfigMapInterface) error {
	configMap, err := buildConfigMap(configPath, loggingconfig.LoggingNamespace, loggingName, level)
	if err != nil {
		return errors.Wrap(err, "buildConfigMap failed")
	}
	existConfig, err := configmaps.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = configmaps.Create(configMap); err != nil && !apierrors.IsAlreadyExists(err) {
				return errors.Wrap(err, "create configmap failed")
			}
			return nil
		}
		return errors.Wrap(err, "list configmap failed")
	}
	newConfigMap := existConfig.DeepCopy()
	newConfigMap.Data = configMap.Data
	if _, err = configmaps.Update(newConfigMap); err != nil {
		errors.Wrap(err, "update configmap failed")
	}
	return nil
}

func RemoveConfigMap(configmaps rv1.ConfigMapInterface) error {
	var errgrp errgroup.Group
	errgrp.Go(func() error {
		return configmaps.Delete(loggingconfig.ClusterLoggingName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return configmaps.Delete(loggingconfig.ProjectLoggingName, &metav1.DeleteOptions{})
	})
	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func buildConfigMap(configPath, namespace, name, level string) (*v1.ConfigMap, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "find cluster logging configuration file failed")
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "read cluster logging configuration file failed")
	}
	configFile := string(buf)

	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"logging-level": level,
			},
		},
		Data: map[string]string{
			level + ".conf": configFile,
		},
	}, nil
}

func newClusterLoggingConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.ClusterLoggingName,
			Namespace: loggingconfig.LoggingNamespace,
			Labels: map[string]string{
				"logging-level": "cluster",
			},
		},
		Data: map[string]string{
			"cluster.conf": "",
		},
	}
}

func newProjectLoggingConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.ProjectLoggingName,
			Namespace: loggingconfig.LoggingNamespace,
			Labels: map[string]string{
				"logging-level": "project",
			},
		},
		Data: map[string]string{
			"project.conf": "",
		},
	}
}
