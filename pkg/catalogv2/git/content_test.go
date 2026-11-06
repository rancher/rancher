package git

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

func Test_loadChart(t *testing.T) {
	const gitURL = "https://git.rancher.io/charts"
	content := []byte("chart-bytes")

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets", "fleet"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "fleet", "fleet-108.0.0+up0.14.0.tgz"), content, 0o644))

	tests := []struct {
		name         string
		chartURL     string
		wantErr      bool
		wantNotFound bool
	}{
		{name: "found", chartURL: "file://assets/fleet/fleet-108.0.0+up0.14.0.tgz"},
		{name: "missing version", chartURL: "file://assets/fleet/fleet-108.0.1+up0.14.1.tgz", wantErr: true, wantNotFound: true},
		{name: "path traversal", chartURL: "file://assets/../../etc/etc-1.0.0.tgz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartVersion := &repo.ChartVersion{
				Metadata: &chart.Metadata{Name: "fleet", Version: "108.0.0+up0.14.0"},
				URLs:     []string{tt.chartURL},
			}
			rc, err := loadChart(dir, gitURL, chartVersion)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantNotFound {
					assert.ErrorIs(t, err, validation.NotFound)
				}
				return
			}
			require.NoError(t, err)
			defer rc.Close()
			got, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, content, got)
		})
	}
}

func Test_relative(t *testing.T) {
	const gitURL = "https://git.rancher.io/charts"
	dir := t.TempDir()
	want := filepath.Join(dir, "assets", "fleet", "fleet-108.0.0.tgz")

	tests := []struct {
		name     string
		chartURL string
		wantErr  bool
	}{
		{name: "relative path", chartURL: "assets/fleet/fleet-108.0.0.tgz"},
		{name: "file URL", chartURL: "file://assets/fleet/fleet-108.0.0.tgz"},
		{name: "git repo URL", chartURL: gitURL + "/assets/fleet/fleet-108.0.0.tgz"},
		{name: "path traversal", chartURL: "../../etc/passwd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relative(dir, gitURL, tt.chartURL)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			want, _ := filepath.Abs(want)
			assert.Equal(t, want, got)
		})
	}
}
