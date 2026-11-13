package main_test

import (
	"bytes"
	"testing"

	main "github.com/rancher/rancher/pkg/codegen/buildconfig"
	"github.com/stretchr/testify/require"
)

var chartYamlConfig = map[string]string{
	"defaultAssetsImage":  "rancher/rancher-assets:v1",
	"defaultShellVersion": "rancher/shell:v2",
	"chartAuditLogImage":  "rancher/mirrored-bci-micro:3",

	"defaultKubeAPIAuthImage": "rancher/kube-api-auth:v4",
	"systemAgentVersion":      "v5",
	"winsAgentVersion":        "v6",
	"machineProvisionImage":   "rancher/machine:v7",
	"defaultSccOperatorImage": "rancher/scc-operator:v8",
}

const chartYamlInput = `apiVersion: v2
version: 0.0.0-VERSION
appVersion: "%APP_VERSION%"
annotations:
  other: value
  helm.sh/images: |
    - name: stale
      image: stale:old

  after: key
`

func TestChartYamlWriterRun(t *testing.T) {
	t.Parallel()

	out := new(bytes.Buffer)
	w := &main.ChartYamlWriter{Config: chartYamlConfig, Chart: bytes.NewBufferString(chartYamlInput), Output: out}
	require.NoError(t, w.Run())

	expected := `apiVersion: v2
version: 0.0.0-VERSION
appVersion: "%APP_VERSION%"
annotations:
  other: value
  helm.sh/images: |
    - name: rancher
      image: rancher/rancher:%APP_VERSION%
    - name: rancher-assets
      image: rancher/rancher-assets:v1
    - name: shell
      image: rancher/shell:v2
    - name: mirrored-bci-micro
      image: rancher/mirrored-bci-micro:3
    - name: kube-api-auth
      image: rancher/kube-api-auth:v4
    - name: rancher-agent
      image: rancher/rancher-agent:%APP_VERSION%
    - name: system-agent
      image: rancher/system-agent:v5
    - name: wins
      image: rancher/wins:v6
      os: windows
    - name: machine
      image: rancher/machine:v7

  after: key
`
	require.Equal(t, expected, out.String())

	// Idempotent.
	again := new(bytes.Buffer)
	w = &main.ChartYamlWriter{Config: chartYamlConfig, Chart: bytes.NewBufferString(out.String()), Output: again}
	require.NoError(t, w.Run())
	require.Equal(t, expected, again.String())
}

func TestChartYamlWriterPrime(t *testing.T) {
	t.Parallel()

	out := new(bytes.Buffer)
	w := &main.ChartYamlWriter{Config: chartYamlConfig, Prime: true, Chart: bytes.NewBufferString(chartYamlInput), Output: out}
	require.NoError(t, w.Run())
	require.Contains(t, out.String(), "image: registry.rancher.com/rancher/rancher:%APP_VERSION%\n")
	require.Contains(t, out.String(), "image: registry.rancher.com/rancher/shell:v2\n")
	require.NotContains(t, out.String(), "image: rancher/")
	require.Contains(t, out.String(), "image: registry.rancher.com/rancher/scc-operator:v8\n")
}

func TestChartYamlWriterNonPrimeOmitsSccOperator(t *testing.T) {
	t.Parallel()

	out := new(bytes.Buffer)
	w := &main.ChartYamlWriter{Config: chartYamlConfig, Chart: bytes.NewBufferString(chartYamlInput), Output: out}
	require.NoError(t, w.Run())
	require.NotContains(t, out.String(), "scc-operator")
}

func TestChartYamlWriterErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing annotation", func(t *testing.T) {
		w := &main.ChartYamlWriter{Config: chartYamlConfig, Chart: bytes.NewBufferString("name: rancher\n"), Output: new(bytes.Buffer)}
		require.ErrorContains(t, w.Run(), "helm.sh/images")
	})
	t.Run("missing build.yaml value", func(t *testing.T) {
		w := &main.ChartYamlWriter{Config: map[string]string{"x": "y"}, Chart: bytes.NewBufferString(chartYamlInput), Output: new(bytes.Buffer)}
		require.ErrorContains(t, w.Run(), "defaultAssetsImage")
	})
	t.Run("missing version", func(t *testing.T) {
		cfg := map[string]string{}
		for k, v := range chartYamlConfig {
			cfg[k] = v
		}
		delete(cfg, "winsAgentVersion")
		w := &main.ChartYamlWriter{Config: cfg, Chart: bytes.NewBufferString(chartYamlInput), Output: new(bytes.Buffer)}
		require.ErrorContains(t, w.Run(), "winsAgentVersion")
	})
	t.Run("image without tag", func(t *testing.T) {
		cfg := map[string]string{}
		for k, v := range chartYamlConfig {
			cfg[k] = v
		}
		cfg["defaultShellVersion"] = "rancher/shell"
		w := &main.ChartYamlWriter{Config: cfg, Chart: bytes.NewBufferString(chartYamlInput), Output: new(bytes.Buffer)}
		require.ErrorContains(t, w.Run(), "no image tag")
	})
	t.Run("nil inputs", func(t *testing.T) {
		require.Error(t, (&main.ChartYamlWriter{Chart: bytes.NewBufferString(chartYamlInput), Output: new(bytes.Buffer)}).Run())
		require.Error(t, (&main.ChartYamlWriter{Config: chartYamlConfig, Output: new(bytes.Buffer)}).Run())
		require.Error(t, (&main.ChartYamlWriter{Config: chartYamlConfig, Chart: bytes.NewBufferString(chartYamlInput)}).Run())
	})
}
