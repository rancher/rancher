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
	kubeConfigFile, err := tempFile("kubeconfig-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeConfigFile.Name())

	yamlFile, err := tempFile("yaml-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(yamlFile.Name())

	if err := ioutil.WriteFile(yamlFile.Name(), yaml, 0600); err != nil {
		return nil, err
	}

	if err := clientcmd.WriteToFile(*kubeConfig, kubeConfigFile.Name()); err != nil {
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

func Drain(ctx context.Context, kubeConfig *clientcmdapi.Config, nodeName string, args []string) ([]byte, string, error) {
	kubeConfigFile, err := tempFile("kubeconfig-")
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(kubeConfigFile.Name())

	if err := clientcmd.WriteToFile(*kubeConfig, kubeConfigFile.Name()); err != nil {
		return nil, "", err
	}

	cmd := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig",
		kubeConfigFile.Name(),
		"drain",
		nodeName)
	cmd.Args = append(cmd.Args, args...)

	var newEnv []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "DISABLE_HTTP2") {
			continue
		}
		newEnv = append(newEnv, env)
	}
	cmd.Env = newEnv
	output, err := cmd.CombinedOutput()
	return output, fmt.Sprint(cmd.Stderr), err
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

func ApplyWithNamespace(yaml []byte, namespace string, kubeConfig *clientcmdapi.Config) ([]byte, error) {
	kubeConfigFile, err := tempFile("kubeconfig-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeConfigFile.Name())

	yamlFile, err := tempFile("yaml-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(yamlFile.Name())

	if err := ioutil.WriteFile(yamlFile.Name(), yaml, 0600); err != nil {
		return nil, err
	}

	if err := clientcmd.WriteToFile(*kubeConfig, kubeConfigFile.Name()); err != nil {
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
