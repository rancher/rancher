package clusterregistrationtoken

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestAgentEnvVars(t *testing.T) {
	c := &v3.Cluster{
		Spec: v3.ClusterSpec{
			ClusterSpecBase: v3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{
						Name:  "HTTPS_PROXY",
						Value: "https://0.0.0.0",
					},
					{
						Name:  "HTTP_PROXY",
						Value: "http://0.0.0.0",
					},
				},
			},
		},
	}
	tests := []struct {
		name     string
		cluster  *v3.Cluster
		envType  EnvType
		expected string
	}{
		{
			name:     "Envvars should be an empty string with a nil cluster",
			cluster:  nil,
			envType:  Linux,
			expected: "",
		},
		{
			name:     "Envvars should be an empty string with no vars set on cluster",
			cluster:  &v3.Cluster{},
			envType:  Linux,
			expected: "",
		},
		{
			name:     "Envvars should be formatted correctly for Linux",
			cluster:  c,
			envType:  Linux,
			expected: "HTTPS_PROXY='https://0.0.0.0' HTTP_PROXY='http://0.0.0.0'",
		},
		{
			name:     "Envvars should be formatted correctly for Docker",
			cluster:  c,
			envType:  Docker,
			expected: "-e HTTPS_PROXY='https://0.0.0.0' -e HTTP_PROXY='http://0.0.0.0'",
		},
		{
			name:     "Envvars should be formatted correctly for PowerShell",
			cluster:  c,
			envType:  PowerShell,
			expected: "$env:HTTPS_PROXY='https://0.0.0.0'; $env:HTTP_PROXY='http://0.0.0.0';",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)

			// act
			evars := AgentEnvVars(tt.cluster, tt.envType)

			// assert
			a.Equal(tt.expected, evars)
		})
	}
}

func clusterWithSystemAgentDir(dir string) *v3.Cluster {
	return &v3.Cluster{
		Spec: v3.ClusterSpec{
			ClusterSpecBase: v3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "HTTPS_PROXY", Value: "https://0.0.0.0"},
					{Name: capr.SystemAgentDataDirEnvVar, Value: dir},
				},
			},
		},
	}
}

func TestAgentEnvVarsSystemAgentDataDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dir        string
		wantLinux  string
		wantPShell string
		wantDocker string
	}{
		{
			name:       "valid directory",
			dir:        "/var/lib/rancher/agent",
			wantLinux:  `HTTPS_PROXY='https://0.0.0.0' CATTLE_AGENT_VAR_DIR='/var/lib/rancher/agent'`,
			wantPShell: `$env:HTTPS_PROXY='https://0.0.0.0'; $env:CATTLE_AGENT_VAR_DIR='/var/lib/rancher/agent';`,
			wantDocker: `-e HTTPS_PROXY='https://0.0.0.0' -e CATTLE_AGENT_VAR_DIR='/var/lib/rancher/agent'`,
		},
		{
			name:       "space",
			dir:        "/data/my dir",
			wantLinux:  `HTTPS_PROXY='https://0.0.0.0' CATTLE_AGENT_VAR_DIR='/data/my dir'`,
			wantPShell: `$env:HTTPS_PROXY='https://0.0.0.0'; $env:CATTLE_AGENT_VAR_DIR='/data/my dir';`,
			wantDocker: `-e HTTPS_PROXY='https://0.0.0.0' -e CATTLE_AGENT_VAR_DIR='/data/my dir'`,
		},
		{
			name:       "command substitution",
			dir:        "/data/$(id)",
			wantLinux:  `HTTPS_PROXY='https://0.0.0.0' CATTLE_AGENT_VAR_DIR='/data/$(id)'`,
			wantPShell: `$env:HTTPS_PROXY='https://0.0.0.0'; $env:CATTLE_AGENT_VAR_DIR='/data/$(id)';`,
			wantDocker: `-e HTTPS_PROXY='https://0.0.0.0' -e CATTLE_AGENT_VAR_DIR='/data/$(id)'`,
		},
		{
			name:       "double quote",
			dir:        `/data/a"b`,
			wantLinux:  `HTTPS_PROXY='https://0.0.0.0' CATTLE_AGENT_VAR_DIR='/data/a"b'`,
			wantPShell: `$env:HTTPS_PROXY='https://0.0.0.0'; $env:CATTLE_AGENT_VAR_DIR='/data/a"b';`,
			wantDocker: `-e HTTPS_PROXY='https://0.0.0.0' -e CATTLE_AGENT_VAR_DIR='/data/a"b'`,
		},
		{
			name:       "single quote",
			dir:        "/data/it's",
			wantLinux:  `HTTPS_PROXY='https://0.0.0.0' CATTLE_AGENT_VAR_DIR='/data/it'\''s'`,
			wantPShell: `$env:HTTPS_PROXY='https://0.0.0.0'; $env:CATTLE_AGENT_VAR_DIR='/data/it''s';`,
			wantDocker: `-e HTTPS_PROXY='https://0.0.0.0' -e CATTLE_AGENT_VAR_DIR='/data/it'\''s'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := clusterWithSystemAgentDir(tt.dir)
			assert.Equal(t, tt.wantLinux, AgentEnvVars(c, Linux))
			assert.Equal(t, tt.wantPShell, AgentEnvVars(c, PowerShell))
			assert.Equal(t, tt.wantDocker, AgentEnvVars(c, Docker))
		})
	}
}

// TestAgentEnvVarsSystemAgentDataDirRunsAsOneWord runs the Linux env prefix under sh and checks that the
// process sees the exact directory and that sh runs no second command.
func TestAgentEnvVarsSystemAgentDataDirRunsAsOneWord(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skipf("/bin/sh is not available: %v", err)
	}

	canary := filepath.Join(t.TempDir(), "canary")
	dirs := []string{
		"/var/lib/rancher/agent",
		"/data/my dir",
		"/data/a\nb",
		"/data/it's",
		`/data/a"b`,
		`/data/a\b`,
		"/data/$(touch " + canary + ")",
		"/data/`touch " + canary + "`",
		`/data/a"; touch ` + canary + `; "`,
		"/data/a;touch " + canary,
	}
	for _, dir := range dirs {
		prefix := AgentEnvVars(clusterWithSystemAgentDir(dir), Linux)
		out, err := exec.Command("/bin/sh", "-c", prefix+` sh -c 'printf %s "$CATTLE_AGENT_VAR_DIR"'`).Output()
		require.NoError(t, err, "dir %q", dir)
		assert.Equal(t, dir, string(out), "dir %q", dir)
		_, err = os.Stat(canary)
		require.True(t, os.IsNotExist(err), "dir %q ran a second command", dir)
	}
}

func TestAgentEnvVarsValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		wantLinux  string
		wantPShell string
		wantDocker string
	}{
		{
			name:       "shell variable and command substitution stay literal",
			value:      "$HOME/$(id)",
			wantLinux:  `X='$HOME/$(id)'`,
			wantPShell: `$env:X='$HOME/$(id)';`,
			wantDocker: `-e X='$HOME/$(id)'`,
		},
		{
			name:       "double dollar and exclamation mark stay literal",
			value:      "https://u:pa$$w!d@proxy",
			wantLinux:  `X='https://u:pa$$w!d@proxy'`,
			wantPShell: `$env:X='https://u:pa$$w!d@proxy';`,
			wantDocker: `-e X='https://u:pa$$w!d@proxy'`,
		},
		{
			name:       "quotes in the value stay part of the value",
			value:      `"abc"`,
			wantLinux:  `X='"abc"'`,
			wantPShell: `$env:X='"abc"';`,
			wantDocker: `-e X='"abc"'`,
		},
		{
			name:       "single quote",
			value:      "it's",
			wantLinux:  `X='it'\''s'`,
			wantPShell: `$env:X='it''s';`,
			wantDocker: `-e X='it'\''s'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &v3.Cluster{Spec: v3.ClusterSpec{ClusterSpecBase: v3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{{Name: "X", Value: tt.value}},
			}}}
			assert.Equal(t, tt.wantLinux, AgentEnvVars(c, Linux))
			assert.Equal(t, tt.wantPShell, AgentEnvVars(c, PowerShell))
			assert.Equal(t, tt.wantDocker, AgentEnvVars(c, Docker))
		})
	}
}

func TestAgentEnvVarsSkipsInvalidNames(t *testing.T) {
	t.Parallel()

	c := &v3.Cluster{Spec: v3.ClusterSpec{ClusterSpecBase: v3.ClusterSpecBase{
		AgentEnvVars: []corev1.EnvVar{
			{Name: "GOOD", Value: "v"},
			{Name: "A B", Value: "v"},
			{Name: "A=1;calc", Value: "v"},
			{Name: "1A", Value: "v"},
			{Name: "$(id)", Value: "v"},
			{Name: "$env:ALSO_GOOD", Value: "v"},
		},
	}}}

	assert.Equal(t, `GOOD='v'`, AgentEnvVars(c, Linux))
	assert.Equal(t, `-e GOOD='v'`, AgentEnvVars(c, Docker))
	assert.Equal(t, `$env:GOOD='v'; $env:ALSO_GOOD='v';`, AgentEnvVars(c, PowerShell))
}
