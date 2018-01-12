package config

import (
	"os"
	"path/filepath"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	namespace         = "kube-system"
	configKey         = "extractedConfig"
	defaultCattleHome = "/var/lib/rancher"
)

type MachineConfig struct {
	configMapClient typedv1.ConfigMapInterface
	baseDir         string
	id              string
	cm              *v1.ConfigMap
}

func NewMachineConfig(client typedv1.ConfigMapsGetter, machine *v3.Machine) (*MachineConfig, error) {
	machineDir, err := buildBaseHostDir(machine.Spec.RequestedHostname)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Created machine storage directory %s", machineDir)

	return &MachineConfig{
		configMapClient: client.ConfigMaps(namespace),
		id:              "cm-" + machine.Name,
		baseDir:         machineDir,
	}, nil
}

func (m *MachineConfig) Dir() string {
	return m.baseDir
}

func (m *MachineConfig) Cleanup() error {
	return os.RemoveAll(m.baseDir)
}

func (m *MachineConfig) Remove() error {
	m.Cleanup()
	return m.configMapClient.Delete(m.id, nil)
}

func (m *MachineConfig) TLSConfig() (*TLSConfig, error) {
	if err := m.loadConfig(); err != nil {
		return nil, err
	}
	return extractTLS(m.cm.Data[configKey])
}

func (m *MachineConfig) IP() (string, error) {
	config, err := m.getConfig()
	if err != nil {
		return "", err
	}

	return convert.ToString(values.GetValueN(config, "Driver", "IPAddress")), nil
}

func (m *MachineConfig) InternalIP() (string, error) {
	config, err := m.getConfig()
	if err != nil {
		return "", err
	}

	return convert.ToString(values.GetValueN(config, "Driver", "PrivateIPAddress")), nil
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

func (m *MachineConfig) getConfig() (map[string]interface{}, error) {
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	data := m.cm.Data[configKey]
	if data == "" {
		return nil, nil
	}

	return extractConfigJSON(data)
}

func buildBaseHostDir(machineName string) (string, error) {
	machineDir := filepath.Join(getWorkDir(), "machines", machineName)
	return machineDir, os.MkdirAll(machineDir, 0740)
}

func getWorkDir() string {
	workDir := os.Getenv("MACHINE_WORK_DIR")
	if workDir == "" {
		workDir = os.Getenv("CATTLE_HOME")
	}
	if workDir == "" {
		workDir = defaultCattleHome
	}
	return filepath.Join(workDir, "machine")
}
