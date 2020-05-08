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
	"github.com/rancher/rancher/pkg/jailer"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
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

func helmInstall(tempDirs *common.HelmPath, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	if !common.IsHelm3(app.Status.HelmVersion) {
		go func() {
			err := common.StartTiller(cont, tempDirs, addr, app.Spec.TargetNamespace)
			if err != nil {
				logrus.Errorf("got error while stopping tiller, error message: %s", err.Error())
			}
		}()
	}
	return common.InstallCharts(tempDirs, addr, app)
}

func helmDelete(tempDirs *common.HelmPath, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	if !common.IsHelm3(app.Status.HelmVersion) {
		go func() {
			err := common.StartTiller(cont, tempDirs, addr, app.Spec.TargetNamespace)
			if err != nil {
				logrus.Errorf("got error while stopping tiller, error message: %s", err.Error())
			}
		}()
	}
	return common.DeleteCharts(tempDirs, addr, app)
}

func (l *Lifecycle) generateTemplates(obj *v3.App) (string, string, *common.HelmPath, error) {
	var appSubDir string
	files := map[string]string{}
	if obj.Spec.ExternalID != "" {
		templateVersionID, templateVersionNamespace, err := common.ParseExternalID(obj.Spec.ExternalID)
		if err != nil {
			return "", "", nil, err
		}

		templateVersion, err := l.TemplateVersionClient.GetNamespaced(templateVersionNamespace, templateVersionID, metav1.GetOptions{})
		if err != nil {
			return "", "", nil, err
		}

		namespace, catalogName, catalogType, _, _, err := common.SplitExternalID(templateVersion.Spec.ExternalID)
		if err != nil {
			return "", "", nil, err
		}
		catalog, err := helmlib.GetCatalog(catalogType, namespace, catalogName, l.CatalogLister, l.ClusterCatalogLister, l.ProjectCatalogLister)
		if err != nil {
			return "", "", nil, err
		}

		helm, err := helmlib.New(catalog)
		if err != nil {
			return "", "", nil, err
		}

		files, err = helm.LoadChart(&templateVersion.Spec, nil)
		if err != nil {
			return "", "", nil, err
		}
		appSubDir = templateVersion.Spec.VersionName
	} else {
		for k, v := range obj.Spec.Files {
			content, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return "", "", nil, err
			}
			files[k] = string(content)
		}
		appSubDir = getAppSubDir(files)
	}

	tempDir, err := createTempDir(obj)
	if err != nil {
		return "", "", nil, err
	}
	if err := writeTempDir(tempDir.FullPath, files); err != nil {
		return "", "", nil, err
	}

	appDir := filepath.Join(tempDir.InJailPath, appSubDir)
	tempDir.AppDirInJail = appDir
	tempDir.AppDirFull = filepath.Join(tempDir.FullPath, appSubDir)

	extraArgs := common.GetExtraArgs(obj)
	setValues, err := common.GenerateAnswerSetValues(obj, tempDir, extraArgs)
	if err != nil {
		return "", "", nil, err
	}

	var commands []string
	var cmd *exec.Cmd
	if common.IsHelm3(obj.Status.HelmVersion) {
		err = l.writeKubeConfig(obj, tempDir.KubeConfigFull, false)
		if err != nil {
			return "", "", nil, err
		}
		commands = append([]string{"template", obj.Name, "--include-crds", appDir, "--namespace", obj.Spec.TargetNamespace, "--kubeconfig", tempDir.KubeConfigInJail}, setValues...)
		cmd = exec.Command(common.HelmV3, commands...)
	} else {
		commands = append([]string{"template", appDir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace}, setValues...)
		cmd = exec.Command(common.HelmV2, commands...)
	}
	sbOut := &bytes.Buffer{}
	sbErr := &bytes.Buffer{}
	cmd.Stdout = sbOut
	cmd.Stderr = sbErr
	cmd, err = common.JailCommand(cmd, tempDir.FullPath)
	if err != nil {
		return "", "", nil, err
	}
	if err := cmd.Start(); err != nil {
		return "", "", nil, errors.Wrapf(err, "start helm template failed. %s", sbErr.String())
	}
	if err := cmd.Wait(); err != nil {
		return "", "", nil, errors.Wrapf(err, "wait helm template failed. %s", sbErr.String())
	}

	// notes.txt
	// do not log --dry-run for helm 3 as it can contain sensitive information
	if common.IsHelm3(obj.Status.HelmVersion) {
		commands = append([]string{"upgrade", "--install", obj.Name, appDir, "--dry-run", "--namespace", obj.Spec.TargetNamespace, "--kubeconfig", tempDir.KubeConfigInJail}, setValues...)
		cmd = exec.Command(common.HelmV3, commands...)
	} else {
		commands = append([]string{"template", appDir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--notes"}, setValues...)
		cmd = exec.Command(common.HelmV2, commands...)
	}
	noteOut := &bytes.Buffer{}
	sbErr = &bytes.Buffer{}
	cmd.Stdout = noteOut
	cmd.Stderr = sbErr
	cmd, err = common.JailCommand(cmd, tempDir.FullPath)
	if err != nil {
		return "", "", nil, err
	}
	if err := cmd.Start(); err != nil {
		return "", "", nil, errors.Wrapf(err, "start helm template rendering notes failed. %s", sbErr.String())
	}
	if err := cmd.Wait(); err != nil {
		return "", "", nil, errors.Wrapf(err, "wait helm template rendering notes failed. %s", sbErr.String())
	}
	template := sbOut.String()
	notes := noteOut.String()
	if common.IsHelm3(obj.Status.HelmVersion) {
		splitNotes := strings.SplitAfter(notes, "NOTES:")
		if len(splitNotes) > 1 {
			notes = splitNotes[1]
		} else {
			notes = ""
		}
	}
	return template, notes, tempDir, nil
}

func createTempDir(obj *v3.App) (*common.HelmPath, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		dir, err := ioutil.TempDir("", "helm-")
		if err != nil {
			return nil, err
		}
		return &common.HelmPath{
			FullPath:         dir,
			InJailPath:       dir,
			KubeConfigFull:   filepath.Join(dir, ".kubeconfig"),
			KubeConfigInJail: filepath.Join(dir, ".kubeconfig"),
			KustomizeFull:    filepath.Join(dir, "kustomize.sh"),
			KustomizeInJail:  filepath.Join(dir, "kustomize.sh"),
		}, nil
	}

	jailDir := obj.Spec.ProjectName + ":" + obj.Name

	err := jailer.CreateJail(jailDir)
	if err != nil {
		return nil, err
	}

	paths := &common.HelmPath{
		FullPath:         filepath.Join(jailer.BaseJailPath, jailDir),
		InJailPath:       "/",
		KubeConfigFull:   filepath.Join(jailer.BaseJailPath, jailDir, ".kubeconfig"),
		KubeConfigInJail: "/.kubeconfig",
		KustomizeFull:    filepath.Join(jailer.BaseJailPath, jailDir, "kustomize.sh"),
		KustomizeInJail:  "/kustomize.sh",
	}

	return paths, nil
}
