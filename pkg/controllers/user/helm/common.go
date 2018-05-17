package helm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"time"

	"encoding/base64"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/templatecontent"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helmName = "helm"
	kubectl  = "kubectl"
	appLabel = "io.cattle.field/appId"
	kcEnv    = "KUBECONFIG"
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

func kubectlApply(template, kubeconfig string, app *v3.App) error {
	file, err := ioutil.TempFile("", "app-template")
	if err != nil {
		return err
	}
	defer os.RemoveAll(file.Name())
	if err := ioutil.WriteFile(file.Name(), []byte(template), 0755); err != nil {
		return err
	}

	command := []string{"apply", "--all", "--overwrite", "-n", app.Spec.TargetNamespace, "-f", file.Name()}
	if app.Spec.Prune {
		command = append(command, "--prune")
	}
	cmd := exec.Command(kubectl, command...)
	cmd.Env = []string{fmt.Sprintf("%s=%s", kcEnv, kubeconfig)}
	sbErr := &bytes.Buffer{}
	cmd.Stdout = os.Stdout
	cmd.Stderr = sbErr
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return errors.Wrapf(err, fmt.Sprintf("Kubectl apply failed. Error: %s", sbErr.String()))
	}

	// wait for resources to be created and try 5 times
	// todo: there is a bug around rolebindings that are being deleted if created outside rancher
	start := 250 * time.Millisecond
	for i := 0; i < 5; i++ {
		time.Sleep(start)
		command = []string{"label", "-n", app.Spec.TargetNamespace, "--overwrite", "-f", file.Name(), fmt.Sprintf("%s=%s", appLabel, app.Name)}
		cmd = exec.Command(kubectl, command...)
		cmd.Env = []string{fmt.Sprintf("%s=%s", kcEnv, kubeconfig)}
		buf := &bytes.Buffer{}
		cmd.Stdout = os.Stdout
		cmd.Stderr = buf
		if err := cmd.Start(); err != nil {
			return err
		}
		if err := cmd.Wait(); err != nil {
			if i == 4 {
				logrus.Warnf("tried 4 times and kubectl label failed. Error: %s", buf.String())
				break
			}
			start = start * 2
			continue
		}
		break
	}

	return nil
}

func kubectlDelete(template, kubeconfig, namespace string) error {
	file, err := ioutil.TempFile("", "app-template")
	if err != nil {
		return err
	}
	defer os.RemoveAll(file.Name())
	if err := ioutil.WriteFile(file.Name(), []byte(template), 0755); err != nil {
		return err
	}
	command := []string{"delete", "--all", "-n", namespace, "-f", file.Name()}
	// try three times and succeed
	start := time.Second * 1
	for i := 0; i < 3; i++ {
		cmd := exec.Command(kubectl, command...)
		cmd.Env = []string{fmt.Sprintf("%s=%s", kcEnv, kubeconfig)}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return err
		}
		if err := cmd.Wait(); err != nil {
			time.Sleep(start)
			start = start * 2
			continue
		}
		break
	}
	return nil
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

func generateTemplates(obj *v3.App, templateVersionClient mgmtv3.TemplateVersionInterface, templateContentClient mgmtv3.TemplateContentInterface) (string, string, error) {
	files := map[string]string{}
	if obj.Spec.ExternalID != "" {
		templateVersionID, err := parseExternalID(obj.Spec.ExternalID)
		if err != nil {
			return "", "", err
		}
		templateVersion, err := templateVersionClient.Get(templateVersionID, metav1.GetOptions{})
		if err != nil {
			return "", "", err
		}
		files, err = convertTemplates(templateVersion.Spec.Files, templateContentClient)
		if err != nil {
			return "", "", err
		}
	} else {
		for k, v := range obj.Spec.Files {
			content, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return "", "", err
			}
			files[k] = string(content)
		}
	}

	tempDir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		return "", "", err
	}
	dir, err := WriteTempDir(tempDir, files)
	defer os.RemoveAll(dir)
	if err != nil {
		return "", "", err
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
		return "", "", errors.Wrapf(err, "helm template failed. %s", sbErr.String())
	}
	if err := cmd.Wait(); err != nil {
		return "", "", errors.Wrapf(err, "helm template failed. %s", sbErr.String())
	}

	// notes.txt
	commands = []string{"template", dir, "--name", obj.Name, "--namespace", obj.Spec.TargetNamespace, "--notes"}
	cmd = exec.Command(helmName, commands...)
	noteOut := &bytes.Buffer{}
	sbErr = &bytes.Buffer{}
	cmd.Stdout = noteOut
	cmd.Stderr = sbErr
	if err := cmd.Start(); err != nil {
		return "", "", errors.Wrapf(err, "helm template --notes failed. %s", sbErr.String())
	}
	if err := cmd.Wait(); err != nil {
		return "", "", errors.Wrapf(err, "helm template --notes failed. %s", sbErr.String())
	}
	template := sbOut.String()
	notes := noteOut.String()
	return template, notes, nil
}

func parseExternalID(externalID string) (string, error) {
	values, err := url.Parse(externalID)
	if err != nil {
		return "", err
	}
	catalog := values.Query().Get("catalog")
	template := values.Query().Get("template")
	version := values.Query().Get("version")
	return strings.Join([]string{catalog, template, version}, "-"), nil
}
