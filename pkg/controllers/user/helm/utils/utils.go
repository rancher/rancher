package utils

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"net/url"

	"github.com/rancher/types/apis/project.cattle.io/v3"
	managementv3 "github.com/rancher/types/client/management/v3"
	"golang.org/x/net/context"
)

const (
	base       = 32768
	end        = 61000
	tillerName = "tiller"
	helmName   = "helm"
)

func RestToRaw(token, clusterID string) KubeConfig {
	rawConfig := KubeConfig{}
	host := fmt.Sprintf("https://localhost:8443/k8s/clusters/%s", clusterID)
	rawConfig.CurrentContext = "default"
	rawConfig.APIVersion = "v1"
	rawConfig.Kind = "Config"
	rawConfig.Clusters = []configCluster{
		{
			Name: "default",
			Cluster: dataCluster{
				Server:                host,
				InsecureSkipVerifyTLS: true,
			},
		},
	}
	rawConfig.Contexts = []configContext{
		{
			Name: "default",
			Context: contextData{
				User:    "admin",
				Cluster: "default",
			},
		},
	}
	rawConfig.Users = []configUser{
		{
			Name: "admin",
			User: userData{
				Token: token,
			},
		},
	}
	return rawConfig
}

func WriteTempDir(rootDir string, files map[string]string) (string, error) {
	for name, content := range files {
		fp := filepath.Join(rootDir, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return "", err
		}
		if err := ioutil.WriteFile(fp, []byte(content), 0755); err != nil {
			return "", err
		}
	}
	for name := range files {
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			return filepath.Join(rootDir, parts[0]), nil
		}
	}
	return "", nil
}

func ConvertTemplates(files []managementv3.File) (map[string]string, error) {
	templates := map[string]string{}
	for _, f := range files {
		content, err := base64.StdEncoding.DecodeString(f.Contents)
		if err != nil {
			return nil, err
		}
		templates[f.Name] = string(content)
	}
	return templates, nil
}

// StartTiller start tiller server and return the listening address of the grpc address
func StartTiller(context context.Context, port, probePort, namespace, kubeConfigPath string) error {
	cmd := exec.Command(tillerName, "--listen", ":"+port, "--probe", ":"+probePort)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "KUBECONFIG", kubeConfigPath), fmt.Sprintf("%s=%s", "TILLER_NAMESPACE", namespace)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	select {
	case <-context.Done():
		return cmd.Process.Kill()
	}
}

func GenerateRandomPort() string {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	port := base + r1.Intn(end-base+1)
	return strconv.Itoa(port)
}

func InstallCharts(rootDir, port string, obj *v3.App) error {
	setValues := []string{}
	if obj.Spec.AnswerValues != "" {
		tempFile, err := ioutil.TempFile("", "temp-answer")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempFile.Name())
		if err := ioutil.WriteFile(tempFile.Name(), []byte(obj.Spec.AnswerValues), 0755); err != nil {
			return err
		}
		setValues = append([]string{"-f"}, tempFile.Name())
	} else if obj.Spec.Answers != nil {
		answers := obj.Spec.Answers
		result := []string{}
		for k, v := range answers {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
		setValues = append([]string{"--set"}, strings.Join(result, ","))
	}
	commands := append([]string{"install", "--namespace", obj.Spec.InstallNamespace, "--name", obj.Name}, setValues...)
	commands = append(commands, rootDir)

	cmd := exec.Command(helmName, commands...)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func DeleteCharts(port string, obj *v3.App) error {
	cmd := exec.Command(helmName, "delete", "--purge", obj.Name)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	combinedOutput, err := cmd.CombinedOutput()
	if combinedOutput != nil && strings.Contains(string(combinedOutput), fmt.Sprintf("Error: release: \"%s\" not found", obj.Name)) {
		return nil
	}
	return err
}

func ParseExternalID(externalID string) (string, error) {
	values, err := url.Parse(externalID)
	if err != nil {
		return "", err
	}
	catalog := values.Query().Get("catalog")
	base := values.Query().Get("base")
	template := values.Query().Get("template")
	version := values.Query().Get("version")
	return strings.Join([]string{catalog, base, template, version}, "-"), nil
}
