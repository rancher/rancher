package helm

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"encoding/base64"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/templatecontent"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helmName    = "helm"
	kubectl     = "kubectl"
	appLabel    = "io.cattle.field/appId"
	failedLabel = "io.cattle.field/failed-revision"
	kcEnv       = "KUBECONFIG"
)

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

func helmInstall(templateDir, kubeconfigPath string, app *v3.App, install bool) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	probeAddr := common.GenerateRandomPort()
	go common.StartTiller(cont, addr, probeAddr, app.Spec.TargetNamespace, kubeconfigPath)
	return common.InstallCharts(templateDir, addr, app, install)
}

func helmDelete(kubeconfigPath string, app *v3.App) error {
	cont, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := common.GenerateRandomPort()
	probeAddr := common.GenerateRandomPort()
	go common.StartTiller(cont, addr, probeAddr, app.Spec.TargetNamespace, kubeconfigPath)
	return common.DeleteCharts(addr, app)
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

func generateTemplates(obj *v3.App, templateVersionClient mgmtv3.TemplateVersionInterface, templateContentClient mgmtv3.TemplateContentInterface) (string, string, string, error) {
	files := map[string]string{}
	if obj.Spec.ExternalID != "" {
		templateVersionID, err := common.ParseExternalID(obj.Spec.ExternalID)
		if err != nil {
			return "", "", "", err
		}
		templateVersion, err := templateVersionClient.Get(templateVersionID, metav1.GetOptions{})
		if err != nil {
			return "", "", "", err
		}
		files, err = convertTemplates(templateVersion.Spec.Files, templateContentClient)
		if err != nil {
			return "", "", "", err
		}
	} else {
		for k, v := range obj.Spec.Files {
			content, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return "", "", "", err
			}
			files[k] = string(content)
		}
	}
	tempDir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		return "", "", "", err
	}
	dir, err := WriteTempDir(tempDir, files)
	if err != nil {
		return "", "", "", err
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
	commands := append([]string{"template", dir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace}, setValues...)

	cmd := exec.Command(helmName, commands...)
	sbOut := &bytes.Buffer{}
	sbErr := &bytes.Buffer{}
	cmd.Stdout = sbOut
	cmd.Stderr = sbErr
	if err := cmd.Start(); err != nil {
		return "", "", "", errors.Wrapf(err, "helm template failed. %s", filterErrorMessage(sbErr.String(), dir, "template-dir"))
	}
	if err := cmd.Wait(); err != nil {
		return "", "", "", errors.Wrapf(err, "helm template failed. %s", filterErrorMessage(sbErr.String(), dir, "template-dir"))
	}

	// notes.txt
	commands = []string{"template", dir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--notes"}
	cmd = exec.Command(helmName, commands...)
	noteOut := &bytes.Buffer{}
	sbErr = &bytes.Buffer{}
	cmd.Stdout = noteOut
	cmd.Stderr = sbErr
	if err := cmd.Start(); err != nil {
		return "", "", "", errors.Wrapf(err, "helm template --notes failed. %s", filterErrorMessage(sbErr.String(), dir, "template-dir"))
	}
	if err := cmd.Wait(); err != nil {
		return "", "", "", errors.Wrapf(err, "helm template --notes failed. %s", filterErrorMessage(sbErr.String(), dir, "template-dir"))
	}
	template := sbOut.String()
	notes := noteOut.String()
	return template, notes, dir, nil
}

// filter error message, replace old with new
func filterErrorMessage(msg, old, new string) string {
	return strings.Replace(msg, old, new, -1)
}
