package common

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	base            = 32768
	end             = 61000
	tillerName      = "tiller"
	helmName        = "helm"
	forceUpgradeStr = "--force"

	systemCatalogName     = "system-library"
	systemDefaultRegistry = "global.systemDefaultRegistry"
)

func ParseExternalID(externalID string) (string, string, error) {
	templateVersionNamespace, catalog, _, template, version, err := SplitExternalID(externalID)
	if err != nil {
		return "", "", err
	}
	return strings.Join([]string{catalog, template, version}, "-"), templateVersionNamespace, nil
}

func SplitExternalID(externalID string) (string, string, string, string, string, error) {
	var templateVersionNamespace, catalog string
	values, err := url.Parse(externalID)
	if err != nil {
		return "", "", "", "", "", err
	}
	catalogWithNamespace := values.Query().Get("catalog")
	catalogType := values.Query().Get("type")
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
	return templateVersionNamespace, catalog, catalogType, template, version, nil
}

func InjectDefaultRegistry(obj *v3.App) {
	if obj.Spec.Answers == nil || obj.Spec.Answers[systemDefaultRegistry] != "" {
		return
	}

	values, err := url.Parse(obj.Spec.ExternalID)
	if err != nil {
		logrus.Errorf("check catalog type failed: %s", err.Error())
	}

	catalogWithNamespace := values.Query().Get("catalog")
	split := strings.SplitN(catalogWithNamespace, "/", 2)
	catalog := split[len(split)-1]

	reg := settings.SystemDefaultRegistry.Get()
	if catalog == systemCatalogName && reg != "" {
		obj.Spec.Answers[systemDefaultRegistry] = reg
	}
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
	InjectDefaultRegistry(obj)
	setValues, err := GenerateAnswerSetValues(obj, rootDir)
	if err != nil {
		return err
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

func GenerateAnswerSetValues(app *v3.App, tempDir string) ([]string, error) {
	setValues := []string{}
	// a user-supplied values file will overridden default values.yaml
	if app.Spec.ValuesYaml != "" {
		valuesYaml := filepath.Join(tempDir, "custom-values.yaml")
		if err := ioutil.WriteFile(valuesYaml, []byte(app.Spec.ValuesYaml), 0755); err != nil {
			return setValues, err
		}
		setValues = append(setValues, "--values", valuesYaml)
	}
	// `--set` values will overridden the user-supplied values.yaml file
	if app.Spec.Answers != nil {
		answers := app.Spec.Answers
		var values = []string{}
		for k, v := range answers {
			values = append(values, fmt.Sprintf("%s=%s", k, v))
		}
		setValues = append(setValues, "--set", strings.Join(values, ","))
	}
	return setValues, nil
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
