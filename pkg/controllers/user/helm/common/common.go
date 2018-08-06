package common

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"bytes"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/project.cattle.io/v3"
)

const (
	base       = 32768
	end        = 61000
	tillerName = "tiller"
	helmName   = "helm"
)

func ParseExternalID(externalID string) (string, error) {
	values, err := url.Parse(externalID)
	if err != nil {
		return "", err
	}
	catalog := values.Query().Get("catalog")
	template := values.Query().Get("template")
	version := values.Query().Get("version")
	return strings.Join([]string{catalog, template, version}, "-"), nil
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

func InstallCharts(rootDir, port string, obj *v3.App, install bool) error {
	setValues := []string{}
	if obj.Spec.Answers != nil {
		answers := obj.Spec.Answers
		result := []string{}
		for k, v := range answers {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
		setValues = append([]string{"--set-string"}, strings.Join(result, ","))
	}
	commands := make([]string, 0)
	if install {
		commands = append([]string{"install", "--namespace", obj.Spec.TargetNamespace, "--name", obj.Name}, setValues...)
	} else {
		commands = append([]string{"upgrade", "--namespace", obj.Spec.TargetNamespace, obj.Name}, setValues...)
	}
	commands = append(commands, rootDir)

	cmd := exec.Command(helmName, commands...)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderrBuf
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to install app %s. error: %s", obj.Name, stderrBuf.String())
	}
	if err := cmd.Wait(); err != nil {
		// if install fail then we delete the release. It can sanctify the error msg.
		if install {
			if err := DeleteCharts(port, obj); err != nil {
				return err
			}
		}
		return errors.Wrapf(err, "failed to install app %s. error: %s", obj.Name, stderrBuf.String())
	}
	return nil
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
