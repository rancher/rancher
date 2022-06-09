package kubectl

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	tmpDir = "./management-state/tmp"
)

func Apply(yaml []byte, kubeConfig *clientcmdapi.Config) ([]byte, error) {
	kubeConfigFile, yamlFile, err := writeData(kubeConfig, yaml)
	defer cleanup(kubeConfigFile, yamlFile)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("kubectl",
		"--kubeconfig",
		kubeConfigFile.Name(),
		"apply",
		"-f",
		yamlFile.Name())
	return runWithHTTP2(cmd)
}

func ApplyWithNamespace(yaml []byte, namespace string, kubeConfig *clientcmdapi.Config) ([]byte, error) {
	kubeConfigFile, yamlFile, err := writeData(kubeConfig, yaml)
	defer cleanup(kubeConfigFile, yamlFile)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("kubectl",
		"--kubeconfig",
		kubeConfigFile.Name(),
		"-n",
		namespace,
		"apply",
		"-f",
		yamlFile.Name())
	return runWithHTTP2(cmd)
}

func RolloutStatusWithNamespace(namespace, name, timeout string, kubeConfig *clientcmdapi.Config) ([]byte, error) {
	kubeConfigFile, err := writeKubeConfig(kubeConfig)
	defer cleanup(kubeConfigFile)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("kubectl",
		"--kubeconfig",
		kubeConfigFile.Name(),
		"-n",
		namespace,
		"rollout",
		"status",
		name,
		"--timeout",
		timeout)
	return runWithHTTP2(cmd)
}

func Delete(yaml []byte, kubeConfig *clientcmdapi.Config) ([]byte, error) {
	kubeConfigFile, yamlFile, err := writeData(kubeConfig, yaml)
	defer cleanup(kubeConfigFile, yamlFile)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("kubectl",
		"--kubeconfig",
		kubeConfigFile.Name(),
		"delete",
		"-f",
		yamlFile.Name())
	return runWithHTTP2(cmd)
}

func Drain(ctx context.Context, kubeConfig *clientcmdapi.Config, nodeName string, args []string) ([]byte, string, error) {
	kubeConfigFile, err := writeKubeConfig(kubeConfig)
	defer cleanup(kubeConfigFile)
	if err != nil {
		return nil, "", err
	}
	cmd := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig",
		kubeConfigFile.Name(),
		"drain",
		nodeName)
	cmd.Args = append(cmd.Args, args...)
	output, err := runWithHTTP2(cmd)
	return output, fmt.Sprint(cmd.Stderr), err
}

func cleanup(files ...*os.File) {
	for _, file := range files {
		if file == nil {
			continue
		}
		os.Remove(file.Name())
	}
}

func writeData(kubeConfig *clientcmdapi.Config, yaml []byte) (*os.File, *os.File, error) {
	kubeConfigFile, err := writeKubeConfig(kubeConfig)
	if err != nil {
		return kubeConfigFile, nil, err
	}
	yamlFile, err := writeYAMLFile(yaml)
	return kubeConfigFile, yamlFile, err
}

func writeYAMLFile(yaml []byte) (*os.File, error) {
	yamlFile, err := tempFile("yaml-")
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(yamlFile.Name(), yaml, 0600); err != nil {
		return nil, err
	}
	return yamlFile, nil
}

func writeKubeConfig(kubeConfig *clientcmdapi.Config) (*os.File, error) {
	kubeConfigFile, err := tempFile("kubeconfig-")
	if err != nil {
		return nil, err
	}
	if err := clientcmd.WriteToFile(*kubeConfig, kubeConfigFile.Name()); err != nil {
		return nil, err
	}
	return kubeConfigFile, nil
}

func tempFile(prefix string) (*os.File, error) {
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err = os.MkdirAll(tmpDir, 0755); err != nil {
			return nil, err
		}
	}

	f, err := ioutil.TempFile(tmpDir, prefix)
	if err != nil {
		return nil, err
	}

	return f, f.Close()
}

func runWithHTTP2(cmd *exec.Cmd) ([]byte, error) {
	var newEnv []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "DISABLE_HTTP2") {
			continue
		}
		newEnv = append(newEnv, env)
	}
	cmd.Env = newEnv
	return cmd.CombinedOutput()
}
