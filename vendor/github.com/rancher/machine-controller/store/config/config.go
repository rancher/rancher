package config

import (
	"os"
	"path/filepath"

	"github.com/rancher/machine-controller/store"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	configKey         = "extractedConfig"
	defaultCattleHome = "/var/lib/rancher"
)

type MachineConfig struct {
	store   *store.GenericEncryptedStore
	baseDir string
	id      string
	cm      map[string]string
}

func NewStore(management *config.ManagementContext) (*store.GenericEncryptedStore, error) {
	return store.NewGenericEncrypedStore("mc-", "", management.Core.Namespaces(""),
		management.K8sClient.CoreV1())
}

func NewMachineConfig(store *store.GenericEncryptedStore, machine *v3.Machine) (*MachineConfig, error) {
	machineDir, err := buildBaseHostDir(machine.Spec.RequestedHostname)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Created machine storage directory %s", machineDir)

	return &MachineConfig{
		store:   store,
		id:      machine.Name,
		baseDir: machineDir,
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
	return m.store.Remove(m.id)
}

func (m *MachineConfig) TLSConfig() (*TLSConfig, error) {
	if err := m.loadConfig(); err != nil {
		return nil, err
	}
	return extractTLS(m.cm[configKey])
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

	if m.cm[configKey] == extractedConfig {
		return nil
	}

	m.cm[configKey] = extractedConfig

	if err := m.store.Set(m.id, m.cm); err != nil {
		m.cm = nil
		return err
	}

	return nil
}

func (m *MachineConfig) Restore() error {
	if err := m.loadConfig(); err != nil {
		return err
	}

	data := m.cm[configKey]
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
		cm = map[string]string{}
	}

	m.cm = cm
	return nil
}

func (m *MachineConfig) getConfigMap() (map[string]string, error) {
	configMap, err := m.store.Get(m.id)
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

	data := m.cm[configKey]
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
