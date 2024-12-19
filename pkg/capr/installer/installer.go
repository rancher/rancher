package installer

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	SystemAgentInstallPath = "/system-agent-install.sh" // corresponding curl -o in package/Dockerfile
	WindowsRke2InstallPath = "/wins-agent-install.ps1"  // corresponding curl -o in package/Dockerfile
)

var (
	localAgentInstallScripts = []string{
		settings.UIPath.Get() + "/assets" + SystemAgentInstallPath,
		"." + SystemAgentInstallPath,
	}
	localWindowsRke2InstallScripts = []string{
		settings.UIPath.Get() + "/assets" + WindowsRke2InstallPath,
		"." + WindowsRke2InstallPath}
)

func installScript(setting settings.Setting, files []string) ([]byte, error) {
	if setting.Get() == setting.Default {
		// no setting override, check for local file first
		for _, f := range files {
			script, err := ioutil.ReadFile(f)
			if err != nil {
				if !os.IsNotExist(err) {
					logrus.Debugf("error pulling system agent installation script %s: %s", f, err)
				}
				continue
			}
			return script, err
		}
		logrus.Debugf("no local installation script found, moving on to url: %s", setting.Get())
	}

	resp, err := http.Get(setting.Get())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func LinuxInstallScript(ctx context.Context, token string, envVars []corev1.EnvVar, defaultHost, _ string) ([]byte, error) {
	data, err := installScript(
		settings.SystemAgentInstallScript,
		localAgentInstallScripts)
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
	if v, ok := ctx.Value(tls.InternalAPI).(bool); ok && v {
		ca = systemtemplate.InternalCAChecksum()
	}
	if ca != "" {
		ca = "CATTLE_CA_CHECKSUM=\"" + ca + "\""
	}
	if token != "" {
		token = "CATTLE_ROLE_NONE=true\nCATTLE_TOKEN=\"" + token + "\""
	}

	// Merge the env vars with the AgentTLSModeStrict
	found := false
	for _, ev := range envVars {
		if ev.Name == "STRICT_VERIFY" {
			found = true // The user has specified `STRICT_VERIFY`, we should not attempt to overwrite it.
		}
	}
	if !found {
		if settings.AgentTLSMode.Get() == settings.AgentTLSModeStrict {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "true",
			})
		} else {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "STRICT_VERIFY",
				Value: "false",
			})
		}
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

func WindowsInstallScript(ctx context.Context, token string, envVars []corev1.EnvVar, defaultHost, dataDir string) ([]byte, error) {
	data, err := installScript(
		settings.WinsAgentInstallScript,
		localWindowsRke2InstallScripts)
	if err != nil {
		return nil, err
	}

	binaryURL := ""
	if settings.WinsAgentVersion.Get() != "" {
		if settings.ServerURL.Get() != "" {
			binaryURL = capr.FormatWindowsEnvVar(corev1.EnvVar{
				Name:  "CATTLE_AGENT_BINARY_BASE_URL",
				Value: fmt.Sprintf("%s/assets", settings.ServerURL.Get()),
			}, false)
		} else if defaultHost != "" {
			binaryURL = capr.FormatWindowsEnvVar(corev1.EnvVar{
				Name:  "CATTLE_AGENT_BINARY_BASE_URL",
				Value: fmt.Sprintf("https://%s/assets", defaultHost),
			}, false)
		}
	}

	csiProxyURL := settings.CSIProxyAgentURL.Get()
	csiProxyVersion := "v1.0.0"
	if settings.CSIProxyAgentVersion.Get() != "" {
		csiProxyVersion = settings.CSIProxyAgentVersion.Get()
		if settings.ServerURL.Get() != "" {
			csiProxyURL = fmt.Sprintf("%s/assets/csi-proxy-%%[1]s.tar.gz", settings.ServerURL.Get())
		} else if defaultHost != "" {
			csiProxyURL = fmt.Sprintf("https://%s/assets/csi-proxy-%%[1]s.tar.gz", defaultHost)
		}
	}

	ca := systemtemplate.CAChecksum()
	if v, ok := ctx.Value(tls.InternalAPI).(bool); ok && v {
		ca = systemtemplate.InternalCAChecksum()
	}

	if ca != "" {
		ca = capr.FormatWindowsEnvVar(corev1.EnvVar{
			Name:  "CATTLE_CA_CHECKSUM",
			Value: ca,
		}, false)
	}

	var tokenEnvVar, cattleRoleNone string
	if token != "" {
		tokenEnvVar = capr.FormatWindowsEnvVar(corev1.EnvVar{
			Name:  "CATTLE_TOKEN",
			Value: token,
		}, false)
		cattleRoleNone = capr.FormatWindowsEnvVar(corev1.EnvVar{
			Name:  "CATTLE_ROLE_NONE",
			Value: "\"true\"",
		}, false)
	}

	envVarBuf := &strings.Builder{}
	for _, envVar := range envVars {
		if envVar.Value == "" {
			continue
		}
		envVarBuf.WriteString(capr.FormatWindowsEnvVar(envVar, false) + "\n")
	}
	server := ""
	if settings.ServerURL.Get() != "" {
		server = capr.FormatWindowsEnvVar(corev1.EnvVar{
			Name:  "CATTLE_SERVER",
			Value: settings.ServerURL.Get(),
		}, false)
	}

	strictVerify := "false"
	if settings.AgentTLSMode.Get() == settings.AgentTLSModeStrict {
		strictVerify = "true"
	}

	return []byte(fmt.Sprintf(`%s

%s
%s
%s
%s
%s

# Enables CSI Proxy
$env:CSI_PROXY_URL = "%s"
$env:CSI_PROXY_VERSION = "%s"
$env:CSI_PROXY_KUBELET_PATH = "C:%s/bin/kubelet.exe"
$env:STRICT_VERIFY = "%s"
%s

Invoke-WinsInstaller @PSBoundParameters
exit 0
`, data, envVarBuf.String(), binaryURL, server, ca, tokenEnvVar, csiProxyURL, csiProxyVersion, dataDir, strictVerify, cattleRoleNone)), nil
}
