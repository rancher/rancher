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

	// Capture and restore global settings to prevent test pollution
	originalServerURL := settings.ServerURL.Get()
	originalCACerts := settings.CACerts.Get()
	t.Cleanup(func() {
		if err := settings.ServerURL.Set(originalServerURL); err != nil {
			t.Errorf("failed to restore ServerURL setting: %v", err)
		}
		if err := settings.CACerts.Set(originalCACerts); err != nil {
			t.Errorf("failed to restore CACerts setting: %v", err)
		}
	})

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

	// Capture and restore global settings to prevent test pollution
	originalServerURL := settings.ServerURL.Get()
	originalCACerts := settings.CACerts.Get()
	t.Cleanup(func() {
		if err := settings.ServerURL.Set(originalServerURL); err != nil {
			t.Errorf("failed to restore ServerURL setting: %v", err)
		}
		if err := settings.CACerts.Set(originalCACerts); err != nil {
			t.Errorf("failed to restore CACerts setting: %v", err)
		}
	})

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

	// Verify environment variables are exported with proper shell escaping (single quotes)
	a.Contains(string(script), "export HTTP_PROXY='http://proxy.example.com:3128'")
	a.Contains(string(script), "export HTTPS_PROXY='http://proxy.example.com:3128'")
	a.Contains(string(script), "export NO_PROXY='127.0.0.0/8,10.0.0.0/8'")

	// Verify other required variables
	a.Contains(string(script), "CATTLE_ROLE_NONE=true")
	a.Contains(string(script), "CATTLE_TOKEN=\"test\"")
	a.Contains(string(script), "CATTLE_SERVER=localhost")
	a.Contains(string(script), fmt.Sprintf("CATTLE_CA_CHECKSUM=\"%s\"", CACertEncoded))

	// Verify shebang
	a.Contains(string(script), "#!/usr/bin/env sh")
}

func TestInstaller_LinuxInstallScript_ShellInjectionPrevention(t *testing.T) {
	a := assert.New(t)

	// Capture and restore global settings
	originalServerURL := settings.ServerURL.Get()
	originalCACerts := settings.CACerts.Get()
	t.Cleanup(func() {
		if err := settings.ServerURL.Set(originalServerURL); err != nil {
			t.Errorf("failed to restore ServerURL setting: %v", err)
		}
		if err := settings.CACerts.Set(originalCACerts); err != nil {
			t.Errorf("failed to restore CACerts setting: %v", err)
		}
	})

	err := settings.ServerURL.Set("localhost")
	a.Nil(err)
	err = settings.CACerts.Set("test")
	a.Nil(err)

	tests := []struct {
		name             string
		envVar           corev1.EnvVar
		shouldContain    string
		shouldNotContain []string
		description      string
	}{
		{
			name: "double quotes in value",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: `value with "quotes"`,
			},
			shouldContain:    `export TEST_VAR='value with "quotes"'`,
			shouldNotContain: []string{`export TEST_VAR="value with "quotes""`},
			description:      "Double quotes should be safely contained within single quotes",
		},
		{
			name: "single quote in value",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: `it's a value`,
			},
			shouldContain:    `export TEST_VAR='it'\''s a value'`,
			shouldNotContain: []string{`export TEST_VAR='it's a value'`},
			description:      "Single quotes should be properly escaped",
		},
		{
			name: "backticks in value",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: "`whoami`",
			},
			shouldContain:    "export TEST_VAR='`whoami`'",
			shouldNotContain: []string{`export TEST_VAR="` + "`whoami`" + `"`},
			description:      "Backticks should not execute commands",
		},
		{
			name: "dollar sign in value",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: "$PATH:/custom/path",
			},
			shouldContain:    "export TEST_VAR='$PATH:/custom/path'",
			shouldNotContain: []string{`export TEST_VAR="$PATH:/custom/path"`},
			description:      "Dollar signs should not trigger variable expansion",
		},
		{
			name: "command substitution attempt",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: "$(rm -rf /)",
			},
			shouldContain:    "export TEST_VAR='$(rm -rf /)'",
			shouldNotContain: []string{`export TEST_VAR="$(rm -rf /)"`},
			description:      "Command substitution should not execute",
		},
		{
			name: "newline in value",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: "line1\nline2",
			},
			shouldContain: "export TEST_VAR='line1\nline2'",
			description:   "Newlines should be safely contained",
		},
		{
			name: "semicolon command separator",
			envVar: corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: "value; rm -rf /",
			},
			shouldContain:    "export TEST_VAR='value; rm -rf /'",
			shouldNotContain: []string{`export TEST_VAR="value; rm -rf /"`},
			description:      "Semicolons should not allow command injection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			script, err := LinuxInstallScript(context.TODO(), "", []corev1.EnvVar{tt.envVar}, "", "")
			a.Nil(err)
			a.NotNil(script)

			scriptStr := string(script)
			a.Contains(scriptStr, tt.shouldContain, tt.description)

			for _, forbidden := range tt.shouldNotContain {
				a.NotContains(scriptStr, forbidden, "Should not use unsafe escaping")
			}
		})
	}
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
			// Non-CATTLE_ envvars must continue to render normally alongside
			// the precedence-controlled CATTLE_* overrides.
			{Name: "HTTP_PROXY", Value: "http://proxy.example.com:3128"},
			{Name: "MyCustomVar", Value: "MyCustomValue"},
		}, "", "/var/lib/rancher/rke2")
	a.Nil(err)
	a.NotNil(script)
	out := string(script)

	// Each user-supplied var is emitted exactly once, in the exported + single-quote
	// escaped form used for agentEnvVars, with the user-provided value. We count the
	// fully-formed export line (rather than a bare "CATTLE_SERVER=" substring) because
	// the embedded install-script body itself references these names in its usage text.
	a.Equal(1, strings.Count(out, fmt.Sprintf("export CATTLE_SERVER='%s'", overrideServer)),
		"CATTLE_SERVER must be emitted once, with the user-provided value")
	a.Equal(1, strings.Count(out, fmt.Sprintf("export CATTLE_AGENT_BINARY_BASE_URL='%s'", overrideAssets)),
		"CATTLE_AGENT_BINARY_BASE_URL must be emitted once, with the user-provided value")
	a.Equal(1, strings.Count(out, fmt.Sprintf("export CATTLE_CA_CHECKSUM='%s'", overrideCAChecksum)),
		"CATTLE_CA_CHECKSUM must be emitted once, with the user-provided value")

	// Non-CATTLE_ envvars must still pass through, rendered in the exported +
	// single-quote escaped form alongside the precedence-controlled CATTLE_* overrides.
	a.Contains(out, "export HTTP_PROXY='http://proxy.example.com:3128'")
	a.Contains(out, "export MyCustomVar='MyCustomValue'")

	// The values otherwise synthesized from global settings must not leak when the
	// user has overridden them via agentEnvVars. The global server-url (and the binary
	// base url derived from it) share the rancher.global host; the global CA checksum
	// is derived from the global CACerts setting.
	a.NotContains(out, "rancher.global.example.com",
		"global server-url must not leak into the script when the user has overridden CATTLE_SERVER")
	a.NotContains(out, fmt.Sprintf("CATTLE_CA_CHECKSUM=\"%s\"", systemtemplate.CAChecksum()),
		"global CA checksum must not be emitted when the user has overridden CATTLE_CA_CHECKSUM")
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
			// Non-CATTLE_ envvars must continue to render normally alongside
			// the precedence-controlled CATTLE_* overrides.
			{Name: "HTTP_PROXY", Value: "http://proxy.example.com:3128"},
			{Name: "MyCustomVar", Value: "MyCustomValue"},
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

	// Non-CATTLE_ envvars must still pass through.
	a.Contains(out, "$env:HTTP_PROXY=\"http://proxy.example.com:3128\"")
	a.Contains(out, "$env:MyCustomVar=\"MyCustomValue\"")

	a.NotContains(out, "rancher.global.example.com",
		"global server-url must not leak into the script when the user has overridden CATTLE_SERVER")
}
