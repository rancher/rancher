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
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

const (
	base                    = 32768
	end                     = 61000
	tillerName              = "rancher-tiller"
	HelmV2                  = "rancher-helm"
	HelmV3                  = "helm_v3"
	forceUpgradeStr         = "--force"
	appLabel                = "io.cattle.field/appId"
	kustomizeTransformFile  = "common-labels.yaml"
	kustomizeTransformSpecs = `
apiVersion: builtin
kind: LabelTransformer
metadata:
  name: common-labels
labels:
  {{.}}
fieldSpecs:
- path: metadata/labels
  create: true
- path: spec/selector
  create: true
  version: v1
  kind: ReplicationController
- path: spec/template/metadata/labels
  create: true
  version: v1
  kind: ReplicationController
- path: spec/selector/matchLabels
  create: true
  kind: Deployment
- path: spec/template/metadata/labels
  create: true
  kind: Deployment
- path: spec/selector/matchLabels
  create: true
  kind: ReplicaSet
- path: spec/template/metadata/labels
  create: true
  kind: ReplicaSet
- path: spec/selector/matchLabels
  create: true
  kind: DaemonSet
- path: spec/template/metadata/labels
  create: true
  kind: DaemonSet
- path: spec/selector/matchLabels
  create: true
  group: apps
  kind: StatefulSet
- path: spec/template/metadata/labels
  create: true
  group: apps
  kind: StatefulSet
- path: spec/volumeClaimTemplates[]/metadata/labels
  create: true
  group: apps
  kind: StatefulSet
- path: spec/template/metadata/labels
  create: true
  group: batch
  kind: Job
- path: spec/jobTemplate/metadata/labels
  create: true
  group: batch
  kind: CronJob
- path: spec/jobTemplate/spec/template/metadata/labels
  create: true
  group: batch
  kind: CronJob
`
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

// Marshal kustomization settings into YAML
type Kustomization struct {
	Resources    []string `json:"resources"`
	Transformers []string `json:"transformers"`
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
	cmd.Env = []string{fmt.Sprintf("%s=%s", "KUBECONFIG", tempDirs.KubeConfigInJail), fmt.Sprintf("%s=%s", "TILLER_NAMESPACE", namespace), fmt.Sprintf("%s=%s", "TILLER_HISTORY_MAX", settings.HelmMaxHistory.Get())}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd, err := jailer.JailCommand(cmd, tempDirs.FullPath)
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
	var commands []string
	if IsHelm3(obj.Status.HelmVersion) {
		err = createKustomizeFiles(tempDirs, obj.Name)
		if err != nil {
			return err
		}
		logrus.Infof("Installing chart using helm version: %s", HelmV3)
		commands = append([]string{"upgrade", "--install", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--kubeconfig", tempDirs.KubeConfigInJail, "--post-renderer", tempDirs.KustomizeInJail, "--history-max", settings.HelmMaxHistory.Get()}, timeoutArgs...)
	} else {
		logrus.Infof("Installing chart using helm version: %s", HelmV2)
		commands = append([]string{"upgrade", "--install", "--namespace", obj.Spec.TargetNamespace, obj.Name}, timeoutArgs...)
	}
	commands = append(commands, setValues...)
	commands = append(commands, tempDirs.AppDirInJail)

	if v32.AppConditionForceUpgrade.IsUnknown(obj) {
		commands = append(commands, forceUpgradeStr)
		// don't leave force recreate on the object
		v32.AppConditionForceUpgrade.True(obj)
	}
	var cmd *exec.Cmd
	// switch userTriggeredAction back
	v32.AppConditionUserTriggeredAction.Unknown(obj)
	if IsHelm3(obj.Status.HelmVersion) {
		cmd = exec.Command(HelmV3, commands...)
	} else {
		cmd = exec.Command(HelmV2, commands...)
	}
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port), fmt.Sprintf("%s=%s", "CATTLE_KUSTOMIZE_YAML", tempDirs.InJailPath)}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderrBuf
	cmd, err = jailer.JailCommand(cmd, tempDirs.FullPath)
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
			if !v32.AppConditionForceUpgrade.IsUnknown(obj) {
				v32.AppConditionForceUpgrade.Unknown(obj)
			}
			return errors.New(v32.AppConditionInstalled.GetMessage(obj))
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

// Convert answers map into a slice of "k=v" formatted values with escaped commas (Required by Helm)
func answersMapToValuesSlice(answers, extraArgs map[string]string) []string {
	var slice []string
	for k, v := range answers {
		if _, ok := extraArgs[k]; ok {
			continue
		}
		slice = append(slice, fmt.Sprintf("%s=%s", k, escapeCommas(v)))
	}
	for k, v := range extraArgs {
		slice = append(slice, fmt.Sprintf("%s=%s", k, escapeCommas(v)))
	}
	return slice
}

func GenerateAnswerSetValues(app *v3.App, tempDir *HelmPath, extraArgs map[string]string) ([]string, error) {
	var values []string
	// A user-supplied values.yaml will override the default values.yaml
	if app.Spec.ValuesYaml != "" {
		custom := "custom-values.yaml"
		valuesYaml := filepath.Join(tempDir.FullPath, custom)
		if err := ioutil.WriteFile(valuesYaml, []byte(app.Spec.ValuesYaml), 0755); err != nil {
			return values, err
		}
		values = append(values, "--values", filepath.Join(tempDir.InJailPath, custom))
	}
	// Values in --set args will override values in the user-supplied values.yaml
	if app.Spec.Answers != nil || extraArgs != nil {
		setValues := answersMapToValuesSlice(app.Spec.Answers, extraArgs)
		values = append(values, "--set", strings.Join(setValues, ","))
	}
	// Values in --set-string args will override values in --set args because
	// Helm gives presedence to the right-most set flag
	if app.Spec.AnswersSetString != nil {
		setStringValues := answersMapToValuesSlice(app.Spec.AnswersSetString, map[string]string{})
		values = append(values, "--set-string", strings.Join(setStringValues, ","))
	}
	return values, nil
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
	cmd, err := jailer.JailCommand(cmd, tempDirs.FullPath)
	if err != nil {
		return err
	}
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil && combinedOutput != nil && strings.Contains(string(combinedOutput), fmt.Sprintf("Error: release: \"%s\" not found", obj.Name)) {
		return nil
	}
	return errors.New(string(combinedOutput))
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
	err := createKustomizeTransformFile(tempDirs, labelValue)
	if err != nil {
		return err
	}
	resources := []string{"all.yaml"}
	transformers := []string{kustomizeTransformFile}
	k := Kustomization{resources, transformers}
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
		err = os.Link("./kustomize.sh", kpath)
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

// generates the kustomization transformers yaml from tmpl that is used by the helm 3 post rendered to
// add the app labels needed to track the workloads with the app deployment
// this is required to avoid the app labels being added on k8s objects selectors
func createKustomizeTransformFile(tempDirs *HelmPath, value string) error {
	ktValue := appLabel + ": " + value
	tmpl := template.New("KustomizeTransform")
	tmpl, err := tmpl.Parse(kustomizeTransformSpecs)
	if err != nil {
		return err
	}
	result := &bytes.Buffer{}
	err = tmpl.Execute(result, ktValue)
	if err != nil {
		return err
	}
	ktPath := filepath.Join(tempDirs.FullPath, "/"+kustomizeTransformFile)
	err = ioutil.WriteFile(ktPath, result.Bytes(), 0644)
	if err != nil {
		return err
	}
	logrus.Debugf("successfully created %s in %s", kustomizeTransformFile, ktPath)
	return nil
}

func IsHelm3(helmName string) bool {
	if helmName == HelmV3 || strings.ToLower(helmName) == "v3" {
		return true
	}
	return false
}
