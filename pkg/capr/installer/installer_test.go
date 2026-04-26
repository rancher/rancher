package installer

import (
	"context"
	"fmt"
	"strings"
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

// TestInstaller_LinuxInstallScript_AgentEnvVarsPrecedence verifies that user-supplied
// agentEnvVars take precedence over values otherwise derived from the global server-url
// setting. Without precedence, the bootstrap script duplicates these vars (user's first,
// then global), and shell-script semantics make the global value win.
func TestInstaller_LinuxInstallScript_AgentEnvVarsPrecedence(t *testing.T) {
	a := assert.New(t)

	a.Nil(settings.ServerURL.Set("https://rancher.global.example.com"))
	a.Nil(settings.CACerts.Set("global-ca-cert-contents"))
	a.Nil(settings.SystemAgentVersion.Set("v0.3.0"))

	overrideServer := "https://rancher.override.example.com"
	overrideAssets := overrideServer + "/assets"
	overrideCAChecksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	script, err := LinuxInstallScript(context.TODO(), "test-token",
		[]corev1.EnvVar{
			{Name: "CATTLE_SERVER", Value: overrideServer},
			{Name: "CATTLE_AGENT_BINARY_BASE_URL", Value: overrideAssets},
			{Name: "CATTLE_CA_CHECKSUM", Value: overrideCAChecksum},
		}, "", "/var/lib/rancher/rke2")
	a.Nil(err)
	a.NotNil(script)
	out := string(script)

	// Each var must appear exactly once, with the user-provided value.
	a.Equal(1, strings.Count(out, "CATTLE_SERVER="),
		"CATTLE_SERVER must not be emitted twice (user value + global override)")
	a.Equal(1, strings.Count(out, "CATTLE_AGENT_BINARY_BASE_URL="),
		"CATTLE_AGENT_BINARY_BASE_URL must not be emitted twice")
	a.Equal(1, strings.Count(out, "CATTLE_CA_CHECKSUM="),
		"CATTLE_CA_CHECKSUM must not be emitted twice")

	a.Contains(out, fmt.Sprintf("CATTLE_SERVER=\"%s\"", overrideServer))
	a.Contains(out, fmt.Sprintf("CATTLE_AGENT_BINARY_BASE_URL=\"%s\"", overrideAssets))
	a.Contains(out, fmt.Sprintf("CATTLE_CA_CHECKSUM=\"%s\"", overrideCAChecksum))

	a.NotContains(out, "rancher.global.example.com",
		"global server-url must not leak into the script when the user has overridden CATTLE_SERVER")
}

// TestInstaller_LinuxInstallScript_FallbackToServerURL verifies that when the user
// does not supply CATTLE_SERVER / CATTLE_AGENT_BINARY_BASE_URL / CATTLE_CA_CHECKSUM,
// the script still falls back to values derived from global settings (existing behavior).
func TestInstaller_LinuxInstallScript_FallbackToServerURL(t *testing.T) {
	a := assert.New(t)

	a.Nil(settings.ServerURL.Set("https://rancher.global.example.com"))
	a.Nil(settings.CACerts.Set("global-ca-cert-contents"))
	a.Nil(settings.SystemAgentVersion.Set("v0.3.0"))

	caChecksum := systemtemplate.CAChecksum()

	script, err := LinuxInstallScript(context.TODO(), "test-token", nil, "", "/var/lib/rancher/rke2")
	a.Nil(err)
	a.NotNil(script)
	out := string(script)

	a.Contains(out, "CATTLE_SERVER=https://rancher.global.example.com")
	a.Contains(out, "CATTLE_AGENT_BINARY_BASE_URL=\"https://rancher.global.example.com/assets\"")
	a.Contains(out, fmt.Sprintf("CATTLE_CA_CHECKSUM=\"%s\"", caChecksum))
}

// TestInstaller_WindowsInstallScript_AgentEnvVarsPrecedence verifies the same precedence
// behavior for the Windows install script.
func TestInstaller_WindowsInstallScript_AgentEnvVarsPrecedence(t *testing.T) {
	a := assert.New(t)

	a.Nil(settings.ServerURL.Set("https://rancher.global.example.com"))
	a.Nil(settings.CACerts.Set("global-ca-cert-contents"))
	a.Nil(settings.WinsAgentVersion.Set("v0.4.0"))

	overrideServer := "https://rancher.override.example.com"
	overrideAssets := overrideServer + "/assets"
	overrideCAChecksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	script, err := WindowsInstallScript(context.TODO(), "test-token",
		[]corev1.EnvVar{
			{Name: "CATTLE_SERVER", Value: overrideServer},
			{Name: "CATTLE_AGENT_BINARY_BASE_URL", Value: overrideAssets},
			{Name: "CATTLE_CA_CHECKSUM", Value: overrideCAChecksum},
		}, "", "/var/lib/rancher/rke2")
	a.Nil(err)
	a.NotNil(script)
	out := string(script)

	a.Equal(1, strings.Count(out, "$env:CATTLE_SERVER="),
		"$env:CATTLE_SERVER must not be emitted twice (user value + global override)")
	a.Equal(1, strings.Count(out, "$env:CATTLE_AGENT_BINARY_BASE_URL="),
		"$env:CATTLE_AGENT_BINARY_BASE_URL must not be emitted twice")
	a.Equal(1, strings.Count(out, "$env:CATTLE_CA_CHECKSUM="),
		"$env:CATTLE_CA_CHECKSUM must not be emitted twice")

	a.Contains(out, fmt.Sprintf("$env:CATTLE_SERVER=\"%s\"", overrideServer))
	a.Contains(out, fmt.Sprintf("$env:CATTLE_AGENT_BINARY_BASE_URL=\"%s\"", overrideAssets))
	a.Contains(out, fmt.Sprintf("$env:CATTLE_CA_CHECKSUM=\"%s\"", overrideCAChecksum))

	a.NotContains(out, "rancher.global.example.com",
		"global server-url must not leak into the script when the user has overridden CATTLE_SERVER")
}
