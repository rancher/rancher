package utils

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	rv1 "github.com/rancher/types/apis/core/v1"
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
		return errors.Wrapf(err, "build configmap %s:%s failed", loggingconfig.LoggingNamespace, loggingName)
	}
	existConfig, err := configmaps.Get(loggingName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if existConfig, err = configmaps.Create(configMap); err != nil && !apierrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "create configmap %s:%s failed", existConfig.Namespace, existConfig.Name)
			}
		} else {
			return errors.Wrapf(err, "get configmap %s:%s failed", loggingconfig.LoggingNamespace, loggingName)
		}
		return errors.Wrap(err, "list configmap failed")
	}
	existConfig.Data = configMap.Data
	if _, err = configmaps.Update(existConfig); err != nil {
		return errors.Wrapf(err, "update configmap %s:%s failed", existConfig.Namespace, existConfig.Name)
	}
	return nil
}

func buildConfigMap(configPath, namespace, name, level string) (*v1.ConfigMap, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "find %s logging configuration file %s failed", level, configPath)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "read %s logging configuration file %s failed", level, configPath)
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
