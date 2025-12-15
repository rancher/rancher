package node

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var regExHyphen = regexp.MustCompile("([a-z])([A-Z])")

var (
	RegExNodeDirEnv      = regexp.MustCompile("^" + nodeDirEnvKey + ".*")
	RegExNodePluginToken = regexp.MustCompile("^" + "MACHINE_PLUGIN_TOKEN=" + ".*")
	RegExNodeDriverName  = regexp.MustCompile("^" + "MACHINE_PLUGIN_DRIVER_NAME=" + ".*")
)

const (
	errorCreatingNode = "Error creating machine: "
	nodeDirEnvKey     = "MACHINE_STORAGE_PATH="
	nodeCmd           = "rancher-machine"
	ec2TagFlag        = "tags"
)

// buildDriverFlags extracts driver-specific configuration from the given configmap and turns it into CLI flags.
func buildDriverFlags(driverName string, configMap map[string]any) []string {
	cmd := make([]string, 0)

	for k, v := range configMap {
		dmField := "--" + driverName + "-" + strings.ToLower(regExHyphen.ReplaceAllString(k, "${1}-${2}"))
		if v == nil {
			continue
		}

		switch v.(type) {
		case float64:
			cmd = append(cmd, dmField, fmt.Sprintf("%v", v))
		case string:
			if v.(string) != "" {
				cmd = append(cmd, dmField, v.(string))
			}
		case bool:
			if v.(bool) {
				cmd = append(cmd, dmField)
			}
		case []interface{}:
			for _, s := range v.([]interface{}) {
				if _, ok := s.(string); ok {
					cmd = append(cmd, dmField, s.(string))
				}
			}
		}
	}

	return cmd
}

func buildEngineOpts(name string, values []string) []string {
	var opts []string
	for _, value := range values {
		if value == "" {
			continue
		}
		opts = append(opts, name, value)
	}
	return opts
}

func mapToSlice(m map[string]string) []string {
	ret := make([]string, len(m))
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func storageOptMapToSlice(m map[string]string) []string {
	ret := make([]string, len(m))
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("storage-opt %s=%s", k, v))
	}
	return ret
}

func logOptMapToSlice(m map[string]string) []string {
	ret := make([]string, len(m))
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("log-opt %s=%s", k, v))
	}
	return ret
}

func initEnviron(nodeDir string) []string {
	env := os.Environ()
	found := false
	for idx, ev := range env {
		if RegExNodeDirEnv.MatchString(ev) {
			env[idx] = nodeDirEnvKey + nodeDir
			found = true
		}
		if RegExNodePluginToken.MatchString(ev) {
			env[idx] = ""
		}
		if RegExNodeDriverName.MatchString(ev) {
			env[idx] = ""
		}
	}
	if !found {
		env = append(env, nodeDirEnvKey+nodeDir)
	}
	return env
}

func getSSHKey(nodeDir, keyPath string, obj *v3.Node) (string, error) {
	keyName := filepath.Base(keyPath)
	if keyName == "" || keyName == "." || keyName == string(filepath.Separator) {
		keyName = "id_rsa"
	}
	if err := waitUntilSSHKey(nodeDir, keyName, obj); err != nil {
		return "", err
	}

	return getSSHPrivateKey(nodeDir, keyName, obj)
}

func filterDockerMessage(msg string, node *v3.Node) (string, error) {
	if strings.Contains(msg, errorCreatingNode) {
		return "", errors.New(msg)
	}
	if strings.Contains(msg, node.Spec.RequestedHostname) {
		return "", nil
	}
	return msg, nil
}

func getSSHPrivateKey(nodeDir, keyName string, node *v3.Node) (string, error) {
	keyPath := filepath.Join(nodeDir, "machines", node.Spec.RequestedHostname, keyName)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", nil
	}
	return string(data), nil
}

func waitUntilSSHKey(nodeDir, keyName string, node *v3.Node) error {
	keyPath := filepath.Join(nodeDir, "machines", node.Spec.RequestedHostname, keyName)
	startTime := time.Now()
	increments := 1
	for {
		if time.Now().After(startTime.Add(15 * time.Second)) {
			return errors.New("Timeout waiting for ssh key")
		}
		if _, err := os.Stat(keyPath); err != nil {
			logrus.Debugf("keyPath not found. The node is probably still provisioning. Sleep %v second", increments)
			time.Sleep(time.Duration(increments) * time.Second)
			increments = increments * 2
			continue
		}
		return nil
	}
}

func setEc2ClusterIDTag(data interface{}, clusterID string) {
	if m, ok := data.(map[string]interface{}); ok {
		tagValue := fmt.Sprintf("kubernetes.io/cluster/%s,owned", clusterID)
		if tags, ok := m[ec2TagFlag]; !ok || convert.ToString(tags) == "" {
			m[ec2TagFlag] = tagValue
		} else {
			m[ec2TagFlag] = convert.ToString(tags) + "," + tagValue
		}
	}
}

func (m *Lifecycle) getKubeConfig(cluster *v3.Cluster) (*clientcmdapi.Config, string, error) {
	user, err := m.systemAccountManager.GetSystemUser(cluster.Name)
	if err != nil {
		return nil, "", err
	}

	tokenPrefix := "node-removal-drain-" + user.Name
	token, err := m.systemTokens.EnsureSystemToken(tokenPrefix, "token for node drain during removal", "agent", user.Name, nil, true)
	if err != nil {
		return nil, "", err
	}

	tokenName, _ := tokens.SplitTokenParts(token)
	return m.clusterManager.KubeConfig(cluster.Name, token), tokenName, nil
}
