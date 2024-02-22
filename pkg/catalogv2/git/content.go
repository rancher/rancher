package git

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/pkg/catalogv2/chart"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"helm.sh/helm/v3/pkg/repo"
)

// Icon will return the icon for a chartName version in a local repository by getting the relative path
func Icon(namespace, name, gitURL string, chartVersion *repo.ChartVersion) (io.ReadCloser, string, error) {
	if len(chartVersion.Icon) == 0 {
		return nil, "", fmt.Errorf("failed to find chartName %s version %s: %w", chartVersion.Name, chartVersion.Version, validation.NotFound)
	}

	dir := RepoDir(namespace, name, gitURL)
	icon := chartVersion.Icon

	file, err := relative(dir, gitURL, icon)
	if err != nil {
		return nil, "", err
	}

	f, err := os.Open(file)
	return f, path.Ext(file), err
}

func Chart(namespace, name, gitURL string, chartVersion *repo.ChartVersion) (io.ReadCloser, error) {
	dir := RepoDir(namespace, name, gitURL)

	if len(chartVersion.URLs) == 0 {
		return nil, fmt.Errorf("failed to find chartName %s version %s: %w", chartVersion.Name, chartVersion.Version, validation.NotFound)
	}

	file, err := relative(dir, gitURL, chartVersion.URLs[0])
	if err != nil {
		return nil, err
	}

	archive, ok, err := chart.LoadArchive(file)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("failed to find chartName %s version %s: %w", chartVersion.Name, chartVersion.Version, validation.NotFound)
	}

	return archive.Open()
}

func relative(base, publicURL, path string) (string, error) {
	if strings.HasPrefix(path, publicURL) {
		path = path[len(publicURL):]
	}
	path = strings.TrimPrefix(path, "file://")

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(filepath.Join(base, path))
	if err != nil {
		return "", err
	}
	_, err = filepath.Rel(baseAbs, fullAbs)
	return fullAbs, err
}
