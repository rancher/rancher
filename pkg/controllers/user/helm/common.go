package helm

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helmName    = "helm"
	appLabel    = "io.cattle.field/appId"
	failedLabel = "io.cattle.field/failed-revision"
)

func writeTempDir(rootDir string, files map[string]string) error {
	for name, content := range files {
		fp := filepath.Join(rootDir, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(fp, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func getAppSubDir(files map[string]string) string {
	var minLen = math.MaxInt32
	var appSubDir string
	for filename := range files {
		dir, file := filepath.Split(filename)
		if strings.EqualFold(file, "Chart.yaml") {
			pathLen := len(filepath.SplitList(dir))
			if minLen > pathLen {
				appSubDir = dir
				minLen = pathLen
			}
		}
	}
	return appSubDir
}

func helmInstall(templateDir, kubeconfigPath string, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	probeAddr := common.GenerateRandomPort()
	go common.StartTiller(cont, addr, probeAddr, app.Spec.TargetNamespace, kubeconfigPath)
	return common.InstallCharts(templateDir, addr, app)
}

func helmDelete(kubeconfigPath string, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	probeAddr := common.GenerateRandomPort()
	go common.StartTiller(cont, addr, probeAddr, app.Spec.TargetNamespace, kubeconfigPath)
	return common.DeleteCharts(addr, app)
}

func (l *Lifecycle) generateTemplates(obj *v3.App) (string, string, string, string, error) {
	var appSubDir string
	files := map[string]string{}
	if obj.Spec.ExternalID != "" {
		templateVersionID, templateVersionNamespace, err := common.ParseExternalID(obj.Spec.ExternalID)
		if err != nil {
			return "", "", "", "", err
		}

		templateVersion, err := l.TemplateVersionClient.GetNamespaced(templateVersionNamespace, templateVersionID, metav1.GetOptions{})
		if err != nil {
			return "", "", "", "", err
		}

		namespace, catalogName, catalogType, _, _, err := common.SplitExternalID(templateVersion.Spec.ExternalID)
		catalog, err := helmlib.GetCatalog(catalogType, namespace, catalogName, l.CatalogLister, l.ClusterCatalogLister, l.ProjectCatalogLister)
		if err != nil {
			return "", "", "", "", err
		}

		helm, err := helmlib.New(catalog)
		if err != nil {
			return "", "", "", "", err
		}

		files, err = helm.LoadChart(&templateVersion.Spec, nil)
		if err != nil {
			return "", "", "", "", err
		}
		appSubDir = templateVersion.Spec.VersionName
	} else {
		for k, v := range obj.Spec.Files {
			content, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return "", "", "", "", err
			}
			files[k] = string(content)
		}
		appSubDir = getAppSubDir(files)
	}

	tempDir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		return "", "", "", "", err
	}
	if err := writeTempDir(tempDir, files); err != nil {
		return "", "", "", tempDir, err
	}

	appDir := filepath.Join(tempDir, appSubDir)

	common.InjectDefaultRegistry(obj)
	setValues, err := common.GenerateAnswerSetValues(obj, tempDir)
	if err != nil {
		return "", "", "", tempDir, err
	}

	commands := append([]string{"template", appDir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace}, setValues...)

	cmd := exec.Command(helmName, commands...)
	sbOut := &bytes.Buffer{}
	sbErr := &bytes.Buffer{}
	cmd.Stdout = sbOut
	cmd.Stderr = sbErr
	if err := cmd.Start(); err != nil {
		return "", "", "", tempDir, errors.Wrapf(err, "helm template failed. %s", filterErrorMessage(sbErr.String(), appDir, "template-dir"))
	}
	if err := cmd.Wait(); err != nil {
		return "", "", "", tempDir, errors.Wrapf(err, "helm template failed. %s", filterErrorMessage(sbErr.String(), appDir, "template-dir"))
	}

	// notes.txt
	commands = append([]string{"template", appDir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--notes"}, setValues...)
	cmd = exec.Command(helmName, commands...)
	noteOut := &bytes.Buffer{}
	sbErr = &bytes.Buffer{}
	cmd.Stdout = noteOut
	cmd.Stderr = sbErr
	if err := cmd.Start(); err != nil {
		return "", "", "", tempDir, errors.Wrapf(err, "helm template --notes failed. %s", filterErrorMessage(sbErr.String(), appDir, "template-dir"))
	}
	if err := cmd.Wait(); err != nil {
		return "", "", "", tempDir, errors.Wrapf(err, "helm template --notes failed. %s", filterErrorMessage(sbErr.String(), appDir, "template-dir"))
	}
	template := sbOut.String()
	notes := noteOut.String()
	return template, notes, appDir, tempDir, nil
}

// filter error message, replace old with new
func filterErrorMessage(msg, old, new string) string {
	return strings.Replace(msg, old, new, -1)
}
