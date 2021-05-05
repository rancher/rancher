package installer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	corev1 "k8s.io/api/core/v1"
)

var (
	defaultSystemAgentInstallScript = "https://raw.githubusercontent.com/ibuildthecloud/system-agent/main/install.sh"
	localAgentInstallScripts        = []string{
		"/usr/share/rancher/ui/assets/system-agent-install.sh",
		"./system-agent-install.sh",
	}
)

func InstallScript(token string, envVars []corev1.EnvVar) ([]byte, error) {
	data, err := installScript()
	if err != nil {
		return nil, err
	}
	binaryURL := ""
	if settings.SystemAgentVersion.Get() != "" && settings.ServerURL.Get() != "" {
		binaryURL = fmt.Sprintf("CATTLE_AGENT_BINARY_BASE_URL=\"%s/assets\"", settings.ServerURL.Get())
	}
	ca := systemtemplate.CAChecksum()
	if ca != "" {
		ca = "CATTLE_CA_CHECKSUM=\"" + ca + "\""
	}
	if token != "" {
		token = "CATTLE_ROLE_NONE=true\nCATTLE_TOKEN=\"" + token + "\""
	}
	envVarBuf := &strings.Builder{}
	for _, envVar := range envVars {
		if envVar.Value == "" {
			continue
		}
		envVarBuf.WriteString(fmt.Sprintf("%s=\"%s\"\n", envVar.Name, envVar.Value))
	}
	return []byte(fmt.Sprintf(`#!/usr/bin/env sh
%s
%s
CATTLE_SERVER="%s"
%s
%s

%s
`, envVarBuf.String(), binaryURL, settings.ServerURL.Get(), ca, token, data)), nil
}

func installScript() ([]byte, error) {
	url := settings.SystemAgentInstallScript.Get()
	if url == "" {
		for _, localAgentInstallScript := range localAgentInstallScripts {
			script, err := ioutil.ReadFile(localAgentInstallScript)
			if !os.IsNotExist(err) {
				return script, err
			}
		}
	}

	if url == "" {
		url = defaultSystemAgentInstallScript
	}

	resp, httpErr := http.Get(url)
	if httpErr != nil {
		return nil, httpErr
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
