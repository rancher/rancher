package node

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func buildAgentCommand(node *v3.Node, dockerRun string) []string {
	drun := strings.Fields(dockerRun)
	cmd := []string{"--native-ssh", "ssh", node.Spec.RequestedHostname}
	cmd = append(cmd, drun...)
	cmd = append(cmd, "-r", "-n", node.Name)
	return cmd
}

func buildCreateCommand(node *v3.Node, configMap map[string]interface{}) []string {
	sDriver := strings.ToLower(node.Status.NodeTemplateSpec.Driver)
	cmd := []string{"create", "-d", sDriver}

	cmd = append(cmd, buildEngineOpts("--engine-install-url", []string{node.Status.NodeTemplateSpec.EngineInstallURL})...)
	cmd = append(cmd, buildEngineOpts("--engine-opt", mapToSlice(node.Status.NodeTemplateSpec.EngineOpt))...)
	cmd = append(cmd, buildEngineOpts("--engine-env", mapToSlice(node.Status.NodeTemplateSpec.EngineEnv))...)
	cmd = append(cmd, buildEngineOpts("--engine-insecure-registry", node.Status.NodeTemplateSpec.EngineInsecureRegistry)...)
	cmd = append(cmd, buildEngineOpts("--engine-label", mapToSlice(node.Status.NodeTemplateSpec.EngineLabel))...)
	cmd = append(cmd, buildEngineOpts("--engine-registry-mirror", node.Status.NodeTemplateSpec.EngineRegistryMirror)...)
	cmd = append(cmd, buildEngineOpts("--engine-storage-driver", []string{node.Status.NodeTemplateSpec.EngineStorageDriver})...)

	for k, v := range configMap {
		dmField := "--" + sDriver + "-" + strings.ToLower(regExHyphen.ReplaceAllString(k, "${1}-${2}"))
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
	logrus.Tracef("create cmd %v", cmd)
	cmd = append(cmd, node.Spec.RequestedHostname)
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
	var ret []string
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func buildCommand(nodeDir string, node *v3.Node, cmdArgs []string) (*exec.Cmd, error) {
	// In dev_mode, don't need jail or reference to jail in command
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		env := initEnviron(nodeDir)
		command := exec.Command(nodeCmd, cmdArgs...)
		command.Env = env
		return command, nil
	}

	command := exec.Command(nodeCmd, cmdArgs...)
	command.Env = []string{
		nodeDirEnvKey + nodeDir,
		"PATH=/usr/bin:/var/lib/rancher/management-state/bin",
	}
	return jailer.JailCommand(command, path.Join(jailer.BaseJailPath, node.Namespace))
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

func startReturnOutput(command *exec.Cmd) (io.ReadCloser, io.ReadCloser, error) {
	readerStdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	readerStderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := command.Start(); err != nil {
		readerStdout.Close()
		readerStderr.Close()
		return nil, nil, err
	}

	return readerStdout, readerStderr, nil
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

func (m *Lifecycle) reportStatus(stdoutReader io.Reader, stderrReader io.Reader, node *v3.Node) (*v3.Node, error) {
	scanner := bufio.NewScanner(stdoutReader)
	for scanner.Scan() {
		msg := scanner.Text()
		if strings.Contains(msg, "To see how to connect") {
			continue
		}
		logrus.Debugf("stdout: %s", msg)
		_, err := filterDockerMessage(msg, node)
		if err != nil {
			return node, err
		}
		logrus.Infof("[node-controller-rancher-machine] %v", msg)
		v32.NodeConditionProvisioned.Message(node, msg)
		// ignore update errors
		if newObj, err := m.nodeClient.Update(node); err == nil {
			node = newObj
		} else {
			node, _ = m.nodeClient.Get(node.Name, metav1.GetOptions{})
		}
	}
	scanner = bufio.NewScanner(stderrReader)
	for scanner.Scan() {
		msg := scanner.Text()
		return node, errors.New(msg)
	}
	return node, nil
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

func nodeExists(nodeDir string, node *v3.Node) (bool, error) {
	command, err := buildCommand(nodeDir, node, []string{"ls", "-q"})
	if err != nil {
		return false, err
	}

	r, err := command.StdoutPipe()
	if err != nil {
		return false, err
	}

	if err = command.Start(); err != nil {
		return false, err
	}
	defer command.Wait()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		foundName := scanner.Text()
		if foundName == node.Spec.RequestedHostname {
			return true, nil
		}
	}
	if err = scanner.Err(); err != nil {
		return false, err
	}

	if err := command.Wait(); err != nil {
		return false, err
	}

	return false, nil
}

func deleteNode(nodeDir string, node *v3.Node) error {
	command, err := buildCommand(nodeDir, node, []string{"rm", "-f", node.Spec.RequestedHostname})
	if err != nil {
		return err
	}
	stdoutReader, stderrReader, err := startReturnOutput(command)
	if err != nil {
		return err
	}
	defer stdoutReader.Close()
	defer stderrReader.Close()
	scanner := bufio.NewScanner(stdoutReader)
	for scanner.Scan() {
		msg := scanner.Text()
		logrus.Infof("[node-controller-rancher-machine] %v", msg)
	}
	scanner = bufio.NewScanner(stderrReader)
	for scanner.Scan() {
		msg := scanner.Text()
		logrus.Warnf("[node-controller-rancher-machine] %v", msg)
	}

	return command.Wait()
}

func getSSHPrivateKey(nodeDir, keyName string, node *v3.Node) (string, error) {
	keyPath := filepath.Join(nodeDir, "machines", node.Spec.RequestedHostname, keyName)
	data, err := ioutil.ReadFile(keyPath)
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

func (m *Lifecycle) getKubeConfig(cluster *v3.Cluster) (*clientcmdapi.Config, error) {
	user, err := m.systemAccountManager.GetSystemUser(cluster.Name)
	if err != nil {
		return nil, err
	}

	token, err := m.userManager.EnsureToken("node-removal-drain-"+user.Name, "token for node drain during removal", "agent", user.Name)
	if err != nil {
		return nil, err
	}

	return m.clusterManager.KubeConfig(cluster.Name, token), nil
}
