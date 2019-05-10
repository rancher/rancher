package nodeconfig

import (
	"os"
	"path/filepath"

	"encoding/json"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/jailer"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	configKey         = "extractedConfig"
	driverKey         = "driverConfig"
	defaultCattleHome = "./management-state"
)

type NodeConfig struct {
	store           *encryptedstore.GenericEncryptedStore
	fullMachinePath string
	jailDir         string
	id              string
	cm              map[string]string
}

func NewStore(namespaceInterface v1.NamespaceInterface, secretsGetter v1.SecretsGetter) (*encryptedstore.GenericEncryptedStore, error) {
	return encryptedstore.NewGenericEncrypedStore("mc-", "", namespaceInterface, secretsGetter)
}

func NewNodeConfig(store *encryptedstore.GenericEncryptedStore, node *v3.Node) (*NodeConfig, error) {
	jailDir, fullMachinePath, err := buildBaseHostDir(node.Spec.RequestedHostname, node.Namespace)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Created node storage directory %s", fullMachinePath)

	return &NodeConfig{
		store:           store,
		id:              node.Name,
		fullMachinePath: fullMachinePath,
		jailDir:         jailDir,
	}, nil
}

func (m *NodeConfig) SetDriverConfig(config string) error {
	if err := m.loadConfig(); err != nil {
		return err
	}
	m.cm[driverKey] = config
	return nil
}

func (m *NodeConfig) DriverConfig() (string, error) {
	if err := m.loadConfig(); err != nil {
		return "", err
	}
	return m.cm[driverKey], nil
}

func (m *NodeConfig) SSHUser() (string, error) {
	if err := m.loadConfig(); err != nil {
		return "", err
	}
	data := map[string]interface{}{}
	if err := json.Unmarshal([]byte(m.cm[driverKey]), &data); err != nil {
		return "", err
	}
	user, _ := data["sshUser"].(string)
	if user == "" {
		user = "root"
	}
	return user, nil
}

func (m *NodeConfig) Dir() string {
	return m.jailDir
}

func (m *NodeConfig) FullDir() string {
	return m.fullMachinePath
}

func (m *NodeConfig) Cleanup() error {
	return os.RemoveAll(m.fullMachinePath)
}

func (m *NodeConfig) Remove() error {
	m.Cleanup()
	return m.store.Remove(m.id)
}

func (m *NodeConfig) TLSConfig() (*TLSConfig, error) {
	if err := m.loadConfig(); err != nil {
		return nil, err
	}
	return extractTLS(m.cm[configKey])
}

func (m *NodeConfig) IP() (string, error) {
	config, err := m.getConfig()
	if err != nil {
		return "", err
	}

	return convert.ToString(values.GetValueN(config, "Driver", "IPAddress")), nil
}

func (m *NodeConfig) InternalIP() (string, error) {
	config, err := m.getConfig()
	if err != nil {
		return "", err
	}

	return convert.ToString(values.GetValueN(config, "Driver", "PrivateIPAddress")), nil
}

func (m *NodeConfig) Save() error {
	extractedConfig, err := compressConfig(m.fullMachinePath)
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

func (m *NodeConfig) Restore() error {
	if err := m.loadConfig(); err != nil {
		return err
	}

	data := m.cm[configKey]
	if data == "" {
		return nil
	}

	return extractConfig(m.fullMachinePath, data)
}

func (m *NodeConfig) loadConfig() error {
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

func (m *NodeConfig) getConfigMap() (map[string]string, error) {
	configMap, err := m.store.Get(m.id)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func (m *NodeConfig) getConfig() (map[string]interface{}, error) {
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	data := m.cm[configKey]
	if data == "" {
		return nil, nil
	}

	return extractConfigJSON(data)
}

func buildBaseHostDir(nodeName string, clusterID string) (string, string, error) {
	var fullMachinePath string
	var jailDir string

	suffix := filepath.Join("node", "nodes", nodeName)
	if dm := os.Getenv("CATTLE_DEV_MODE"); dm != "" {
		fullMachinePath = filepath.Join(defaultCattleHome, suffix)
		jailDir = fullMachinePath
	} else {
		fullMachinePath = filepath.Join(jailer.BaseJailPath, clusterID, "management-state", suffix)
		jailDir = filepath.Join("/management-state", suffix)
	}

	return jailDir, fullMachinePath, os.MkdirAll(fullMachinePath, 0740)
}
