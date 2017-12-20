package config

import (
	"os"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	namespace = "kube-system"
	configKey = "extractedConfig"
)

type MachineConfig struct {
	configMapClient typedv1.ConfigMapInterface
	baseDir         string
	id              string
	cm              *v1.ConfigMap
}

func NewMachineConfig(client typedv1.ConfigMapsGetter, baseDir, machineID string) *MachineConfig {
	return &MachineConfig{
		configMapClient: client.ConfigMaps(namespace),
		id:              "cm-" + machineID,
		baseDir:         baseDir,
	}
}

func (m *MachineConfig) Cleanup() error {
	return os.RemoveAll(m.baseDir)
}

func (m *MachineConfig) Save() error {
	extractedConfig, err := compressConfig(m.baseDir)
	if err != nil {
		return err
	}

	if err := m.loadConfig(); err != nil {
		return err
	}

	if m.cm.Data[configKey] == extractedConfig {
		return nil
	}

	m.cm.Data[configKey] = extractedConfig

	newCm, err := m.configMapClient.Update(m.cm)
	if err != nil {
		m.cm = nil
		return err
	}

	m.cm = newCm
	return nil
}

func (m *MachineConfig) Restore() error {
	if err := m.loadConfig(); err != nil {
		return err
	}

	data := m.cm.Data[configKey]
	if data == "" {
		return nil
	}

	return extractConfig(m.baseDir, data)
}

func (m *MachineConfig) loadConfig() error {
	if m.cm != nil {
		return nil
	}

	cm, err := m.getConfigMap()
	if err != nil {
		return err
	}

	if cm == nil {
		cm = &v1.ConfigMap{}
		cm.Name = m.id

		cm, err = m.configMapClient.Create(cm)
		if err != nil {
			return err
		}
		cm.Data = map[string]string{}
	}

	m.cm = cm
	return nil
}

func (m *MachineConfig) getConfigMap() (*v1.ConfigMap, error) {
	configMap, err := m.configMapClient.Get(m.id, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return configMap, nil
}
