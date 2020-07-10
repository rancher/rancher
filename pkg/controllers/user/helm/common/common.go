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
	"syscall"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/jailer"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/namespace"
	"github.com/sirupsen/logrus"
)

const (
	base            = 32768
	end             = 61000
	tillerName      = "rancher-tiller"
	HelmV2          = "rancher-helm"
	HelmV3          = "helm_v3"
	forceUpgradeStr = "--force"
	maxHistory      = "10"
)

type HelmPath struct {
	// /opt/jail/<app-name>
	FullPath string
	// /
	InJailPath string
	// /opt/jail/<app-name>/.kubeconfig
	KubeConfigFull string
	// /.kubeconfig
	KubeConfigInJail string
	// /opt/jail/<app-name>/<app-sub>
	AppDirFull string
	// /<app-sub>
	AppDirInJail string
	// /opt/jail/<app-name>/kustomize.sh
	KustomizeFull string
	// /kustomize.sh
	KustomizeInJail string
}

//Labels that need added for kustomization
type Label struct {
	AppLabel string `json:"io.cattle.field/appId"`
}

//Marshal kustomization settings into YAML
type Kustomization struct {
	CommonLabel Label    `json:"commonLabels"`
	Resrouces   []string `json:"resources"`
}

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

// StartTiller start tiller server and return the listening address of the grpc address
func StartTiller(context context.Context, tempDirs *HelmPath, port, namespace string) error {
	probePort := GenerateRandomPort()
	cmd := exec.Command(tillerName, "--listen", ":"+port, "--probe-listen", ":"+probePort)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "KUBECONFIG", tempDirs.KubeConfigInJail), fmt.Sprintf("%s=%s", "TILLER_NAMESPACE", namespace), fmt.Sprintf("%s=%s", "TILLER_HISTORY_MAX", maxHistory)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd, err := JailCommand(cmd, tempDirs.FullPath)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	select {
	case <-context.Done():
		logrus.Debug("Stopping Tiller")
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

func InstallCharts(tempDirs *HelmPath, port string, obj *v3.App) error {
	logrus.Debugf("InstallCharts - helm path info %+v\n", tempDirs)
	extraArgs := GetExtraArgs(obj)
	timeoutArgs := getTimeoutArgs(obj)
	setValues, err := GenerateAnswerSetValues(obj, tempDirs, extraArgs)
	if err != nil {
		return err
	}
	commands := make([]string, 0)
	if IsHelm3(obj.Status.HelmVersion) {
		err = createKustomizeFiles(tempDirs, obj.Name)
		if err != nil {
			return err
		}
		logrus.Infof("Installing chart using helm version: %s", HelmV3)
		commands = append([]string{"upgrade", "--install", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--kubeconfig", tempDirs.KubeConfigInJail, "--post-renderer", tempDirs.KustomizeInJail, "--history-max", maxHistory}, timeoutArgs...)
	} else {
		logrus.Infof("Installing chart using helm version: %s", HelmV2)
		commands = append([]string{"upgrade", "--install", "--namespace", obj.Spec.TargetNamespace, obj.Name}, timeoutArgs...)
	}
	commands = append(commands, setValues...)
	commands = append(commands, tempDirs.AppDirInJail)

	if v3.AppConditionForceUpgrade.IsUnknown(obj) {
		commands = append(commands, forceUpgradeStr)
		// don't leave force recreate on the object
		v3.AppConditionForceUpgrade.True(obj)
	}
	var cmd *exec.Cmd
	// switch userTriggeredAction back
	v3.AppConditionUserTriggeredAction.Unknown(obj)
	if IsHelm3(obj.Status.HelmVersion) {
		cmd = exec.Command(HelmV3, commands...)
	} else {
		cmd = exec.Command(HelmV2, commands...)
	}
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port), fmt.Sprintf("%s=%s", "CATTLE_KUSTOMIZE_YAML", tempDirs.InJailPath)}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderrBuf
	cmd, err = JailCommand(cmd, tempDirs.FullPath)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return errors.Errorf("failed to install app %s. %s", obj.Name, stderrBuf.String())
	}
	if err := cmd.Wait(); err != nil {
		// if the first install failed, the second install would have error message like `has no deployed releases`, then the
		// original error is masked. We need to filter out the message and always return the last one if error matches this pattern
		if strings.Contains(stderrBuf.String(), "has no deployed releases") {
			if !v3.AppConditionForceUpgrade.IsUnknown(obj) {
				v3.AppConditionForceUpgrade.Unknown(obj)
			}
			return errors.New(v3.AppConditionInstalled.GetMessage(obj))
		}
		return errors.Errorf("failed to install app %s. %s", obj.Name, stderrBuf.String())
	}
	return nil
}

// getTimeoutArgs generates the appropriate arguments for helm depending on the app's timeout and wait fields
func getTimeoutArgs(app *v3.App) []string {
	var timeoutArgs []string

	if app.Spec.Wait {
		timeoutArgs = append(timeoutArgs, "--wait")
	}
	if app.Status.HelmVersion == HelmV3 {
		timeoutArgs = append(timeoutArgs, "--timeout", fmt.Sprintf("%ss", strconv.Itoa(app.Spec.Timeout)))
	} else {
		timeoutArgs = append(timeoutArgs, "--timeout", strconv.Itoa(app.Spec.Timeout))
	}
	return timeoutArgs
}

func GenerateAnswerSetValues(app *v3.App, tempDir *HelmPath, extraArgs map[string]string) ([]string, error) {
	setValues := []string{}
	// a user-supplied values file will overridden default values.yaml
	if app.Spec.ValuesYaml != "" {
		custom := "custom-values.yaml"
		valuesYaml := filepath.Join(tempDir.FullPath, custom)
		if err := ioutil.WriteFile(valuesYaml, []byte(app.Spec.ValuesYaml), 0755); err != nil {
			return setValues, err
		}
		setValues = append(setValues, "--values", filepath.Join(tempDir.InJailPath, custom))
	}
	// `--set` values will overridden the user-supplied values.yaml file
	if app.Spec.Answers != nil || extraArgs != nil {
		answers := app.Spec.Answers
		var values = []string{}
		for k, v := range answers {
			if _, ok := extraArgs[k]; ok {
				continue
			}
			// helm will only accept escaped commas in values
			escapedValue := fmt.Sprintf("%s=%s", k, escapeCommas(v))
			values = append(values, escapedValue)
		}
		for k, v := range extraArgs {
			escapedValue := fmt.Sprintf("%s=%s", k, escapeCommas(v))
			values = append(values, escapedValue)
		}
		setValues = append(setValues, "--set", strings.Join(values, ","))
	}
	return setValues, nil
}

func DeleteCharts(tempDirs *HelmPath, port string, obj *v3.App) error {
	var cmd *exec.Cmd
	if IsHelm3(obj.Status.HelmVersion) {
		logrus.Infof("Deleting chart using helm version: %s", HelmV3)
		cmd = exec.Command(HelmV3, "uninstall", obj.Name, "--kubeconfig", tempDirs.KubeConfigInJail, "--namespace", obj.Spec.TargetNamespace)
	} else {
		logrus.Infof("Deleting chart using helm version: %s", HelmV2)
		cmd = exec.Command(HelmV2, "delete", "--purge", obj.Name)
	}
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd, err := JailCommand(cmd, tempDirs.FullPath)
	if err != nil {
		return err
	}
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil && combinedOutput != nil && strings.Contains(string(combinedOutput), fmt.Sprintf("Error: release: \"%s\" not found", obj.Name)) {
		return nil
	}
	return errors.New(string(combinedOutput))
}

func JailCommand(cmd *exec.Cmd, jailPath string) (*exec.Cmd, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return cmd, nil
	}

	cred, err := jailer.GetUserCred()
	if err != nil {
		return nil, errors.WithMessage(err, "get user cred error")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = cred
	cmd.SysProcAttr.Chroot = jailPath
	cmd.Env = jailer.WhitelistEnvvars(cmd.Env)
	cmd.Env = append(cmd.Env, "PWD=/")
	cmd.Dir = "/"
	return cmd, nil
}

// escapeCommas will escape the commas in a string, unless helm would identify it as a list
func escapeCommas(value string) string {
	if len(value) == 0 {
		return value
	}
	// as far as helm is concerned, an answer starting with '{' is a list
	// commas in lists are not escaped for users as they are also used as delimiters
	if strings.HasPrefix(value, "{") {
		return value
	}
	return strings.Replace(value, ",", "\\,", -1)
}

// generates the kustomization yaml that is used by the helm 3 post rendered to
// add the app labels needed to track the workloads with the app deployment
func createKustomizeFiles(tempDirs *HelmPath, labelValue string) error {
	var resources []string
	resources = append(resources, "all.yaml")
	k := Kustomization{Label{labelValue}, resources}
	y, err := yaml.Marshal(k)
	if err != nil {
		return err
	}
	kpath := filepath.Join(tempDirs.FullPath, "/kustomization.yaml")
	err = ioutil.WriteFile(kpath, y, 0644)
	if err != nil {
		return err
	}
	logrus.Debugf("successfully created kustomization.yaml in %s", kpath)

	// set file link to ensure kustomize.sh works in dev move without having to copy
	// the file to the "injail" location
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		kpath = filepath.Join(tempDirs.InJailPath, "/kustomize.sh")
		err = os.Link("./package/kustomize.sh", kpath)
		if err != nil {
			err = os.Link(filepath.Join(os.Getenv("DAPPER_SOURCE"), "package/kustomize.sh"), kpath)
			if err != nil {
				return err
			}
		}
		logrus.Debugf("successfully linked kustomize.sh to %s", kpath)
	}
	return nil
}

func IsHelm3(helmName string) bool {
	if helmName == HelmV3 || strings.ToLower(helmName) == "v3" {
		return true
	}
	return false
}
