package installer

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestInstaller_WindowsInstallScript(t *testing.T) {
	// arrange
	CACert := "uibesrwguiobsdrujigbsduigbuidbg"
	a := assert.New(t)

	// act
	err := settings.ServerURL.Set("localhost")
	a.Nil(err)

	err = settings.CACerts.Set(CACert)
	a.Nil(err)

	CACertEncoded := systemtemplate.CAChecksum()

	agentVar1 := corev1.EnvVar{
		Name:  "TestEnvVar1",
		Value: "TestEnvVarValue",
	}

	agentVar2 := corev1.EnvVar{
		Name:  "TestEnvVar2",
		Value: "TestEnvVarValue",
	}

	formattedAgentVar1 := capr.FormatWindowsEnvVar(agentVar1, false)
	formattedAgentVar2 := capr.FormatWindowsEnvVar(agentVar2, false)

	script, err := WindowsInstallScript(context.TODO(), "test", []corev1.EnvVar{
		agentVar1,
		agentVar2,
	}, "localhost", "/var/lib/rancher/rke2")

	// assert
	a.Nil(err)
	a.NotNil(script)
	a.Contains(string(script), "$env:CATTLE_TOKEN=\"test\"")
	a.Contains(string(script), "$env:CATTLE_ROLE_NONE=\"true\"")
	a.Contains(string(script), "$env:CATTLE_SERVER=\"localhost\"")
	a.Contains(string(script), fmt.Sprintf("$env:CATTLE_CA_CHECKSUM=\"%s\"", CACertEncoded))
	a.Contains(string(script), "$env:CSI_PROXY_URL")
	a.Contains(string(script), "$env:CSI_PROXY_VERSION")
	a.Contains(string(script), "$env:CSI_PROXY_KUBELET_PATH")
	a.Contains(string(script), "$env:TestEnvVar1")
	a.Contains(string(script), "$env:TestEnvVar2")

	// ensure agent env vars are not on the same line
	a.NotContains(string(script), formattedAgentVar1+formattedAgentVar2)
}

func TestInstaller_LinuxInstallScript(t *testing.T) {
	// arrange
	CACert := "uibesrwguiobsdrujigbsduigbuidbg"
	a := assert.New(t)

	// act
	err := settings.ServerURL.Set("localhost")
	a.Nil(err)

	err = settings.CACerts.Set(CACert)
	a.Nil(err)

	CACertEncoded := systemtemplate.CAChecksum()

	agentVar1 := corev1.EnvVar{
		Name:  "HTTP_PROXY",
		Value: "http://proxy.example.com:3128",
	}

	agentVar2 := corev1.EnvVar{
		Name:  "HTTPS_PROXY",
		Value: "http://proxy.example.com:3128",
	}

	agentVar3 := corev1.EnvVar{
		Name:  "NO_PROXY",
		Value: "127.0.0.0/8,10.0.0.0/8",
	}

	script, err := LinuxInstallScript(context.TODO(), "test", []corev1.EnvVar{
		agentVar1,
		agentVar2,
		agentVar3,
	}, "localhost", "")

	// assert
	a.Nil(err)
	a.NotNil(script)

	// Verify environment variables are exported (not just assigned)
	a.Contains(string(script), "export HTTP_PROXY=\"http://proxy.example.com:3128\"")
	a.Contains(string(script), "export HTTPS_PROXY=\"http://proxy.example.com:3128\"")
	a.Contains(string(script), "export NO_PROXY=\"127.0.0.0/8,10.0.0.0/8\"")

	// Verify other required variables
	a.Contains(string(script), "CATTLE_ROLE_NONE=true")
	a.Contains(string(script), "CATTLE_TOKEN=\"test\"")
	a.Contains(string(script), "CATTLE_SERVER=localhost")
	a.Contains(string(script), fmt.Sprintf("CATTLE_CA_CHECKSUM=\"%s\"", CACertEncoded))

	// Verify shebang
	a.Contains(string(script), "#!/usr/bin/env sh")
}
