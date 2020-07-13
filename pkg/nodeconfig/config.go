package nodeconfig

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/jailer"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
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
	logrus.Debugf("Cleaning up [%s]", m.fullMachinePath)
	return os.RemoveAll(m.fullMachinePath)
}

func (m *NodeConfig) Remove() error {
	m.Cleanup()
	logrus.Debugf("Removing [%v]", m.id)
	return m.store.Remove(m.id)
}

func (m *NodeConfig) TLSConfig() (*TLSConfig, error) {
	if err := m.loadConfig(); err != nil {
		return nil, err
	}
	return extractTLS(m.cm[configKey])
}

func (m *NodeConfig) SSHKeyPath() (string, error) {
	config, err := m.getConfig()
	if err != nil {
		return "", err
	}

	return convert.ToString(values.GetValueN(config, "Driver", "SSHKeyPath")), nil
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

// UpdateAmazonAuth updates the machine config.json file on disk with the most
// recent version of creds from the cloud credential
// This code should not be updated or duplicated - it should be deleted once
// https://github.com/rancher/rancher/issues/24541 is implemented
func (m *NodeConfig) UpdateAmazonAuth(rawConfig interface{}) (bool, error) {
	var update bool
	c := convert.ToMapInterface(rawConfig)

	machines := filepath.Join(m.fullMachinePath, "machines")
	logrus.Debugf("[UpdateAmazonAuth] machine path %v", machines)
	files, err := ioutil.ReadDir(machines)
	if err != nil {
		// There aren't any machines, nothing to update
		if os.IsNotExist(err) {
			return update, nil
		}
		return update, err
	}

	for _, file := range files {
		if file.IsDir() {
			configPath := filepath.Join(machines, file.Name(), "config.json")
			b, err := ioutil.ReadFile(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					// config.json doesn't exist, no changes needed
					continue
				}
				return update, err
			}

			logrus.Debugf("[UpdateAmazonAuth] config file found, path %v", configPath)

			result := make(map[string]interface{})

			if err := json.Unmarshal(b, &result); err != nil {
				return update, errors.Wrap(err, "error unmarshaling machine config")
			}

			if _, ok := result["Driver"]; !ok {
				logrus.Debug("[UpdateAmazonAuth] config file does not have Data key")
				// No Driver config so no changes to be made
				continue
			}

			driverConfig := convert.ToMapInterface(result["Driver"])

			if _, ok := driverConfig["AccessKey"]; ok {
				if driverConfig["AccessKey"] != c["accessKey"] {
					logrus.Debug("[UpdateAmazonAuth] update access key")
					driverConfig["AccessKey"] = c["accessKey"]
					update = true
				}
			}

			if _, ok := driverConfig["SecretKey"]; ok {
				if driverConfig["SecretKey"] != c["secretKey"] {
					logrus.Debug("[UpdateAmazonAuth] update secret key")
					driverConfig["SecretKey"] = c["secretKey"]
					update = true
				}

			}

			if update {
				result["Driver"] = driverConfig

				out, err := json.Marshal(result)
				if err != nil {
					return update, errors.WithMessage(err, "error marshaling new machine config")
				}

				if err := ioutil.WriteFile(configPath, out, 0600); err != nil {
					return update, errors.WithMessage(err, "error writing  new machine config")
				}
			}

		}
	}

	return update, nil
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
	if k8serror.IsNotFound(err) {
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
