package main_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	main "github.com/rancher/rancher/pkg/codegen/buildconfig"
	"github.com/stretchr/testify/require"
)

func TestChartValuesWriterRun(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"chartAuditLogImage":  "rancher/mirrored-bci-micro:16.0-15.11",
		"defaultChartsImage":  "rancher/rancher-charts:v0.1.0-rc.1",
		"defaultShellVersion": "rancher/shell:v0.8.0-rc.2",
	}

	chartInput := `auditLog:
  enabled: false
  image:
    repository: "rancher/old-audit"
    tag: old-tag

chartsImage:
  repository: rancher/old-charts
  tag: v0.0.1

postDelete:
  enabled: true
  image:
    repository: rancher/old-shell
    tag: v0.1.0

preUpgrade:
  image:
    repository: rancher/old-shell
    tag: v0.1.0
`

	expected := `auditLog:
  enabled: false
  image:
    repository: "rancher/mirrored-bci-micro"
    tag: 16.0-15.11

chartsImage:
  repository: rancher/rancher-charts
  tag: v0.1.0-rc.1

postDelete:
  enabled: true
  image:
    repository: rancher/shell
    tag: v0.8.0-rc.2

preUpgrade:
  image:
    repository: rancher/shell
    tag: v0.8.0-rc.2
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	require.Equal(t, expected, got)
}

func TestChartValuesWriterPreservesBlankLines(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"defaultShellVersion": "rancher/shell:v0.8.0",
	}

	chartInput := `# Comment before section

postDelete:
  enabled: true

  image:
    repository: rancher/old
    tag: old

  timeout: 120

preUpgrade:
  image:
    repository: rancher/old
    tag: old

# Another section
other:
  value: foo
`

	expected := `# Comment before section

postDelete:
  enabled: true

  image:
    repository: rancher/shell
    tag: v0.8.0

  timeout: 120

preUpgrade:
  image:
    repository: rancher/shell
    tag: v0.8.0

# Another section
other:
  value: foo
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	require.Equal(t, expected, got)
}

func TestChartValuesWriterPreservesQuoting(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"chartAuditLogImage": "rancher/audit:latest",
	}

	chartInput := `auditLog:
  image:
    repository: "rancher/old"
    tag: old-unquoted
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	// Repository should stay quoted, tag should stay unquoted
	require.Contains(t, got, `repository: "rancher/audit"`)
	require.Contains(t, got, "tag: latest\n")
	require.NotContains(t, got, `tag: "latest"`)
}

func TestChartValuesWriterNoChanges(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"defaultShellVersion": "rancher/shell:v0.8.0",
	}

	chartInput := `postDelete:
  image:
    repository: rancher/shell
    tag: v0.8.0

preUpgrade:
  image:
    repository: rancher/shell
    tag: v0.8.0
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	// Should write exact same content when values already match
	require.Equal(t, chartInput, got)
}

func TestChartValuesWriterPartialConfig(t *testing.T) {
	t.Parallel()

	// Only update some values, not all
	cfg := map[string]string{
		"defaultShellVersion": "rancher/shell:v1.1.0",
	}

	chartInput := `auditLog:
  image:
    repository: rancher/audit
    tag: old

postDelete:
  image:
    repository: rancher/shell
    tag: v0.0.1

preUpgrade:
  image:
    repository: rancher/shell
    tag: v0.0.1
`

	expected := `auditLog:
  image:
    repository: rancher/audit
    tag: old

postDelete:
  image:
    repository: rancher/shell
    tag: v1.1.0

preUpgrade:
  image:
    repository: rancher/shell
    tag: v1.1.0
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	require.Equal(t, expected, got)
}

func TestChartValuesWriterFailsWithBadConfiguration(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"defaultShellVersion": "rancher/shell:v0.8.0",
	}
	chartInput := `postDelete:
  image:
    repository: rancher/shell
    tag: v0.1.0
`
	output := new(bytes.Buffer)

	tests := []struct {
		name   string
		config map[string]string
		chart  io.Reader
		output io.Writer
	}{
		{
			name:   "nil config",
			config: nil,
			chart:  bytes.NewBufferString(chartInput),
			output: output,
		},
		{
			name:   "nil chart",
			config: cfg,
			chart:  nil,
			output: output,
		},
		{
			name:   "nil output",
			config: cfg,
			chart:  bytes.NewBufferString(chartInput),
			output: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := main.ChartValuesWriter{
				Config: test.config,
				Chart:  test.chart,
				Output: test.output,
			}
			require.Error(t, w.Run())
		})
	}
}

func TestChartValuesWriterFailsWithMissingPath(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"defaultShellVersion": "rancher/shell:v1.0.0",
	}

	// Chart missing the postDelete section entirely
	chartInput := `auditLog:
  image:
    repository: rancher/audit
    tag: old
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}

	err := w.Run()
	require.Error(t, err)
	require.Contains(t, err.Error(), "postDelete")
	require.Contains(t, err.Error(), "not found")
}

func TestChartValuesWriterHandlesInvalidImageFormat(t *testing.T) {
	t.Parallel()

	// Image without colon separator (invalid format)
	cfg := map[string]string{
		"defaultShellVersion": "rancher-shell-v0.8.0", // No colon
	}

	chartInput := `postDelete:
  image:
    repository: rancher/shell
    tag: v0.1.0
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}

	// Should not error, just skip updating (len(parts) != 2)
	require.NoError(t, w.Run())

	got := out.String()
	// Values should be unchanged
	require.Equal(t, chartInput, got)
}

func TestChartValuesWriterHandlesComplexYAML(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		"defaultShellVersion": "rancher/shell:v0.9.0",
	}

	chartInput := `# Top level comment
global:
  cattle:
    psp:
      enabled: false

# Audit log section
auditLog:
  enabled: false
  level: 0

# Post-delete hook with inline comments
postDelete:
  enabled: true  # Keep enabled
  image:
    # Image registry can be overridden
    repository: rancher/old  # Old image
    tag: v0.1.0  # Old tag
    pullPolicy: IfNotPresent
  timeout: 120

# Pre-upgrade hook
preUpgrade:
  image:
    repository: rancher/old
    tag: v0.1.0

# Unrelated section that should not be touched
other:
  nested:
    deep:
      value: foo
`

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartValuesWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()

	// Should preserve all comments
	require.Contains(t, got, "# Top level comment")
	require.Contains(t, got, "# Keep enabled")
	require.Contains(t, got, "# Image registry can be overridden")

	// Should update the values
	require.Contains(t, got, "repository: rancher/shell")
	require.Contains(t, got, "tag: v0.9.0")

	// Should preserve unrelated sections
	require.Contains(t, got, "value: foo")

	// Should preserve blank lines (count them)
	inputLines := strings.Split(chartInput, "\n")
	outputLines := strings.Split(got, "\n")

	// Count blank lines in input
	inputBlankLines := 0
	for _, line := range inputLines {
		if strings.TrimSpace(line) == "" {
			inputBlankLines++
		}
	}

	// Count blank lines in output
	outputBlankLines := 0
	for _, line := range outputLines {
		if strings.TrimSpace(line) == "" {
			outputBlankLines++
		}
	}

	// Should have same number of blank lines
	require.Equal(t, inputBlankLines, outputBlankLines, "blank line count should be preserved")
}
