package nodeconfig

import (
	"os"
	"path/filepath"

	"encoding/json"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	configKey         = "extractedConfig"
	driverKey         = "driverConfig"
	defaultCattleHome = "./management-state"
)

type NodeConfig struct {
	store   *encryptedstore.GenericEncryptedStore
	baseDir string
	id      string
	cm      map[string]string
}

func NewStore(management *config.ManagementContext) (*encryptedstore.GenericEncryptedStore, error) {
	return encryptedstore.NewGenericEncrypedStore("mc-", "", management.Core.Namespaces(""),
		management.K8sClient.CoreV1())
}

func NewNodeConfig(store *encryptedstore.GenericEncryptedStore, node *v3.Node) (*NodeConfig, error) {
	nodeDir, err := buildBaseHostDir(node.Spec.RequestedHostname)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Created node storage directory %s", nodeDir)

	return &NodeConfig{
		store:   store,
		id:      node.Name,
		baseDir: nodeDir,
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
	return m.baseDir
}

func (m *NodeConfig) Cleanup() error {
	return os.RemoveAll(m.baseDir)
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

func (m *NodeConfig) Restore() error {
	if err := m.loadConfig(); err != nil {
		return err
	}

	data := m.cm[configKey]
	if data == "" {
		return nil
	}

	return extractConfig(m.baseDir, data)
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

func buildBaseHostDir(nodeName string) (string, error) {
	nodeDir := filepath.Join(getWorkDir(), "nodes", nodeName)
	return nodeDir, os.MkdirAll(nodeDir, 0740)
}

func getWorkDir() string {
	workDir := os.Getenv("MACHINE_WORK_DIR")
	if workDir == "" {
		workDir = os.Getenv("CATTLE_HOME")
	}
	if workDir == "" {
		workDir = defaultCattleHome
	}
	return filepath.Join(workDir, "node")
}
