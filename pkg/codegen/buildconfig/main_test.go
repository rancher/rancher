package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChartTarget(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		wantDir       string
		wantRepoFiles bool
	}{
		{name: "default", wantDir: "chart", wantRepoFiles: true},
		{name: "empty override", env: map[string]string{chartDirEnv: ""}, wantDir: "chart", wantRepoFiles: true},
		{name: "override", env: map[string]string{chartDirEnv: "build/chart-prime"}, wantDir: "build/chart-prime", wantRepoFiles: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, repoFiles := chartTarget(func(k string) string { return tt.env[k] })
			require.Equal(t, tt.wantDir, dir)
			require.Equal(t, tt.wantRepoFiles, repoFiles)
		})
	}
}

func writeChartCopy(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYamlFixture), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(valuesFixture), 0644))
	return dir
}

const valuesFixture = `auditLog:
  image:
    repository: old
    tag: old
postDelete:
  image:
    repository: old
    tag: old
preUpgrade:
  image:
    repository: old
    tag: old
assetsImage:
  repository: old
  tag: old
`

const chartYamlFixture = `apiVersion: v2
annotations:
  helm.sh/images: |
    - name: stale
      image: stale:old
`

var targetConfig = map[string]string{
	"defaultAssetsImage":      "rancher/rancher-assets:v1",
	"defaultShellVersion":     "rancher/shell:v2",
	"chartAuditLogImage":      "rancher/mirrored-bci-micro:3",
	"defaultKubeAPIAuthImage": "rancher/kube-api-auth:v4",
	"systemAgentVersion":      "v5",
	"winsAgentVersion":        "v6",
	"machineProvisionImage":   "rancher/machine:v7",
}

func TestUpdateChartYamlInDir(t *testing.T) {
	dir := writeChartCopy(t)
	require.NoError(t, updateChartYaml(targetConfig, dir))

	got, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(got), "rancher/rancher-assets:v1")
	require.NotContains(t, string(got), "stale")
}

func TestUpdateChartValuesInDir(t *testing.T) {
	dir := writeChartCopy(t)
	before, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
	require.NoError(t, err)

	require.NoError(t, updateChartValues(targetConfig, dir))

	after, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
	require.NoError(t, err)
	require.NotEqual(t, string(before), string(after))
}
