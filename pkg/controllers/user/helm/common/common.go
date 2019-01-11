package common

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/project.cattle.io/v3"
)

const (
	base            = 32768
	end             = 61000
	tillerName      = "tiller"
	helmName        = "helm"
	forceUpgradeStr = "--force"
)

func ParseExternalID(externalID string) (string, string, error) {
	var templateVersionNamespace, catalog string
	values, err := url.Parse(externalID)
	if err != nil {
		return "", "", err
	}
	catalogWithNamespace := values.Query().Get("catalog")
	template := values.Query().Get("template")
	version := values.Query().Get("version")
	split := strings.SplitN(catalogWithNamespace, "/", 2)
	if len(split) == 2 {
		templateVersionNamespace = split[0]
		catalog = split[1]
	}
	//pre-upgrade setups will have global catalogs, where externalId field on templateversions won't have namespace.
	// since these are global catalogs, we can default to global namespace
	if templateVersionNamespace == "" {
		templateVersionNamespace = namespace.GlobalNamespace
		catalog = catalogWithNamespace
	}
	return strings.Join([]string{catalog, template, version}, "-"), templateVersionNamespace, nil
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
	for {
		port := base + r1.Intn(end-base+1)
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			continue
		}
		ln.Close()
		return strconv.Itoa(port)
	}
}

func InstallCharts(rootDir, port string, obj *v3.App) error {
	setValues := []string{}
	if obj.Spec.Answers != nil {
		answers := obj.Spec.Answers
		result := []string{}
		for k, v := range answers {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
		setValues = append([]string{"--set"}, strings.Join(result, ","))
	}
	commands := make([]string, 0)
	commands = append([]string{"upgrade", "--install", "--namespace", obj.Spec.TargetNamespace, obj.Name}, setValues...)
	commands = append(commands, rootDir)

	if v3.AppConditionForceUpgrade.IsUnknown(obj) {
		commands = append(commands, forceUpgradeStr)
	}

	cmd := exec.Command(helmName, commands...)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderrBuf
	if err := cmd.Start(); err != nil {
		return errors.Errorf("failed to install app %s. %s", obj.Name, stderrBuf.String())
	}
	if err := cmd.Wait(); err != nil {
		// if the first install failed, the second install would have error message like `has no deployed releases`, then the
		// original error is masked. We need to filter out the message and always return the last one if error matches this pattern
		if strings.Contains(stderrBuf.String(), "has no deployed releases") {
			return errors.New(v3.AppConditionInstalled.GetMessage(obj))
		}
		return errors.Errorf("failed to install app %s. %s", obj.Name, stderrBuf.String())
	}
	return nil
}

func DeleteCharts(port string, obj *v3.App) error {
	cmd := exec.Command(helmName, "delete", "--purge", obj.Name)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil && combinedOutput != nil && strings.Contains(string(combinedOutput), fmt.Sprintf("Error: release: \"%s\" not found", obj.Name)) {
		return nil
	}
	return errors.New(string(combinedOutput))
}
