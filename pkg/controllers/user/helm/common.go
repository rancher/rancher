package helm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/templatecontent"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helmName    = "helm"
	appLabel    = "io.cattle.field/appId"
	failedLabel = "io.cattle.field/failed-revision"
)

func WriteTempDir(tempDirs *common.HelmPath, files map[string]string) error {
	for name, content := range files {
		fp := filepath.Join(tempDirs.FullPath, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(fp, []byte(content), 0755); err != nil {
			return err
		}
	}
	for name := range files {
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			tempDirs.AppDirFull = filepath.Join(tempDirs.FullPath, parts[0])
			tempDirs.AppDirInJail = filepath.Join(tempDirs.InJailPath, parts[0])
			return nil
		}
	}
	return nil
}

func helmInstall(tempDirs *common.HelmPath, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	go common.StartTiller(cont, tempDirs, addr, app.Spec.TargetNamespace)
	return common.InstallCharts(tempDirs, addr, app)
}

func helmDelete(tempDirs *common.HelmPath, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	go common.StartTiller(cont, tempDirs, addr, app.Spec.TargetNamespace)
	return common.DeleteCharts(tempDirs, addr, app)
}

func convertTemplates(files map[string]string, templateContentClient mgmtv3.TemplateContentInterface) (map[string]string, error) {
	templates := map[string]string{}
	for name, tag := range files {
		data, err := templatecontent.GetTemplateFromTag(tag, templateContentClient)
		if err != nil {
			continue
		}
		templates[name] = data
	}
	return templates, nil
}

func generateTemplates(obj *v3.App, templateVersionClient mgmtv3.TemplateVersionInterface, templateContentClient mgmtv3.TemplateContentInterface) (string, string, *common.HelmPath, error) {
	files := map[string]string{}
	if obj.Spec.ExternalID != "" {
		templateVersionID, err := common.ParseExternalID(obj.Spec.ExternalID)
		if err != nil {
			return "", "", nil, err
		}
		templateVersion, err := templateVersionClient.Get(templateVersionID, metav1.GetOptions{})
		if err != nil {
			return "", "", nil, err
		}
		files, err = convertTemplates(templateVersion.Spec.Files, templateContentClient)
		if err != nil {
			return "", "", nil, err
		}
	} else {
		for k, v := range obj.Spec.Files {
			content, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return "", "", nil, err
			}
			files[k] = string(content)
		}
	}
	tempDirs, err := createTempDir(obj)
	if err != nil {
		return "", "", nil, err
	}

	err = WriteTempDir(tempDirs, files)
	if err != nil {
		return "", "", nil, err
	}

	setValues := []string{}
	if obj.Spec.Answers != nil {
		answers := obj.Spec.Answers
		result := []string{}
		for k, v := range answers {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
		setValues = append([]string{"--set"}, strings.Join(result, ","))
	}
	commands := append([]string{"template", tempDirs.AppDirInJail, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace}, setValues...)

	cmd := exec.Command(helmName, commands...)
	sbOut := &bytes.Buffer{}
	sbErr := &bytes.Buffer{}
	cmd.Stdout = sbOut
	cmd.Stderr = sbErr
	cmd, err = common.JailCommand(cmd, tempDirs.FullPath)
	if err != nil {
		return "", "", nil, err
	}

	if err := cmd.Start(); err != nil {
		return "", "", nil, errors.Wrapf(err, "helm template failed. %s", filterErrorMessage(sbErr.String(), tempDirs.AppDirInJail, "template-dir"))
	}
	if err := cmd.Wait(); err != nil {
		return "", "", nil, errors.Wrapf(err, "helm template failed. %s", filterErrorMessage(sbErr.String(), tempDirs.AppDirInJail, "template-dir"))
	}

	// notes.txt
	commands = append([]string{"template", tempDirs.AppDirInJail, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--notes"}, setValues...)
	cmd = exec.Command(helmName, commands...)
	noteOut := &bytes.Buffer{}
	sbErr = &bytes.Buffer{}
	cmd.Stdout = noteOut
	cmd.Stderr = sbErr
	cmd, err = common.JailCommand(cmd, tempDirs.FullPath)
	if err != nil {
		return "", "", nil, err
	}

	if err := cmd.Start(); err != nil {
		return "", "", nil, errors.Wrapf(err, "helm template --notes failed. %s", filterErrorMessage(sbErr.String(), tempDirs.AppDirInJail, "template-dir"))
	}
	if err := cmd.Wait(); err != nil {
		return "", "", nil, errors.Wrapf(err, "helm template --notes failed. %s", filterErrorMessage(sbErr.String(), tempDirs.AppDirInJail, "template-dir"))
	}
	template := sbOut.String()
	notes := noteOut.String()
	return template, notes, tempDirs, nil
}

// filter error message, replace old with new
func filterErrorMessage(msg, old, new string) string {
	return strings.Replace(msg, old, new, -1)
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
		}, nil
	}

	err := jailer.CreateJail(obj.Name)
	if err != nil {
		return nil, err
	}

	paths := &common.HelmPath{
		FullPath:         filepath.Join(jailer.BaseJailPath, obj.Name),
		InJailPath:       "/",
		KubeConfigFull:   filepath.Join(jailer.BaseJailPath, obj.Name, ".kubeconfig"),
		KubeConfigInJail: "/.kubeconfig",
	}

	return paths, nil
}
