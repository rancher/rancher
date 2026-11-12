package git

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/pkg/catalogv2/chart"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// Icon will return the icon for a chartName version in a local repository by getting the relative path
func Icon(namespace, name, gitURL string, chartVersion *repo.ChartVersion) (io.ReadCloser, string, error) {
	if len(chartVersion.Icon) == 0 {
		return nil, "", fmt.Errorf("failed to find chartName %s version %s: %w", chartVersion.Name, chartVersion.Version, validation.NotFound)
	}

	if !IsCheckedOut(name, gitURL) {
		return nil, "", ErrNotCheckedOut
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
	if !IsCheckedOut(name, gitURL) {
		return nil, ErrNotCheckedOut
	}
	return loadChart(RepoDir(namespace, name, gitURL), gitURL, chartVersion)
}

func loadChart(dir, gitURL string, chartVersion *repo.ChartVersion) (io.ReadCloser, error) {
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

// chartAssetPath returns the conventional relative path where a packaged chart archive
// is stored in a repository checkout (assets/<name>/<name>-<version>.tgz).
func chartAssetPath(name, version string) string {
	return path.Join("assets", name, fmt.Sprintf("%s-%s.tgz", name, version))
}

// LocalChart returns a chart version from the local repository checkout when its index entry points
// at a remote URL (such as an oci:// or http(s):// reference). The chart is looked up at the
// conventional assets/<name>/<name>-<version>.tgz path and verified against chartVersion.Digest.
func LocalChart(namespace, name, gitURL string, chartVersion *repo.ChartVersion) (io.ReadCloser, error) {
	if !IsCheckedOut(name, gitURL) {
		return nil, ErrNotCheckedOut
	}
	assetPath := chartAssetPath(chartVersion.Name, chartVersion.Version)
	return openChartArchive(RepoDir(namespace, name, gitURL), gitURL, assetPath, chartVersion.Digest)
}

// openChartArchive resolves a relative path inside the repository checkout, opens the chart archive,
// and verifies its SHA-256 digest against expectedDigest if provided.
func openChartArchive(dir, gitURL, chartPath, expectedDigest string) (io.ReadCloser, error) {
	file, err := relative(dir, gitURL, chartPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to find chart at %s: %w", file, validation.NotFound)
		}
		return nil, err
	}

	if expectedDigest != "" {
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return nil, err
		}
		actual := hex.EncodeToString(h.Sum(nil))
		expected := strings.TrimPrefix(strings.ToLower(expectedDigest), "sha256:")
		if !strings.EqualFold(actual, expected) {
			f.Close()
			return nil, fmt.Errorf("chart %s has digest %s, index expects %s: %w", file, actual, expectedDigest, validation.NotFound)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
	}

	return f, nil
}

func relative(base, publicURL, path string) (string, error) {
	path = strings.TrimPrefix(path, publicURL)
	path = strings.TrimPrefix(path, "file://")

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(filepath.Join(base, path))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, fullAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("invalid file path [%s]", path)
	}

	return fullAbs, nil
}
