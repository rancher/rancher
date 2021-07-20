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
	defaultSystemAgentInstallScript  = "https://raw.githubusercontent.com/rancher/system-agent/main/install.sh"
	defaultWindowsAgentInstallScript = "https://raw.githubusercontent.com/rancher/rke2/master/windows/rke2-install.ps1"
	localAgentInstallScripts         = []string{
		"/usr/share/rancher/ui/assets/system-agent-install.sh",
		"./system-agent-install.sh",
	}
	localWindowsAgentInstallScripts = []string{
		"./windows-agent-install.ps1",
	}
)

func InstallScript(token string, envVars []corev1.EnvVar, defaultHost string) ([]byte, error) {
	data, err := installScript()
	if err != nil {
		return nil, err
	}
	binaryURL := ""
	if settings.SystemAgentVersion.Get() != "" {
		if settings.ServerURL.Get() != "" {
			binaryURL = fmt.Sprintf("CATTLE_AGENT_BINARY_BASE_URL=\"%s/assets\"", settings.ServerURL.Get())
		} else if defaultHost != "" {
			binaryURL = fmt.Sprintf("CATTLE_AGENT_BINARY_BASE_URL=\"https://%s/assets\"", defaultHost)
		}
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
	server := ""
	if settings.ServerURL.Get() != "" {
		server = fmt.Sprintf("CATTLE_SERVER=%s", settings.ServerURL.Get())
	}
	return []byte(fmt.Sprintf(`#!/usr/bin/env sh
%s
%s
%s
%s
%s

%s
`, envVarBuf.String(), binaryURL, server, ca, token, data)), nil
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

func WindowsInstallScript(token string, envVars []corev1.EnvVar, defaultHost string) ([]byte, error) {
	data, err := windowsInstallScript()
	if err != nil {
		return nil, err
	}

	ca := systemtemplate.CAChecksum()
	if ca != "" {
		ca = "$env:CATTLE_CA_CHECKSUM=\"" + ca + "\""
	}
	if token != "" {
		token = "$env:CATTLE_ROLE_NONE=true\n$env:CATTLE_TOKEN=\"" + token + "\""
	}
	envVarBuf := &strings.Builder{}
	for _, envVar := range envVars {
		if envVar.Value == "" {
			continue
		}
		envVarBuf.WriteString(fmt.Sprintf("$env:%s=\"%s\"\n", envVar.Name, envVar.Value))
	}
	server := ""
	if settings.ServerURL.Get() != "" {
		server = fmt.Sprintf("$env:CATTLE_SERVER=%s", settings.ServerURL.Get())
	}

	return []byte(fmt.Sprintf(`
%s
%s
%s
%s

%s
`, envVarBuf.String(), server, ca, token, data)), nil
}

func windowsInstallScript() ([]byte, error) {
	url := settings.WindowsAgentInstallScript.Get()
	if url == "" {
		for _, localWindowsInstallScript := range localWindowsAgentInstallScripts {
			script, err := ioutil.ReadFile(localWindowsInstallScript)
			if !os.IsNotExist(err) {
				return script, err
			}
		}
	}

	if url == "" {
		url = defaultWindowsAgentInstallScript
	}

	resp, httpErr := http.Get(url)
	if httpErr != nil {
		return nil, httpErr
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
