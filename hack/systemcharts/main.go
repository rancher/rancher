// This tool parses build.yaml to extract the necessary information to retrieve all "system charts".
// Reads the corresponding index.yaml, finds the pinned versions and downloads the chart tarballs to the output directory.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1"
	"sigs.k8s.io/yaml"
)

var (
	buildYamlPath = flag.String("build-yaml", "", "path to build yaml")
	outputDir     = flag.String("output-dir", "", "path to output directory")
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	flag.Parse()
	if err := run(ctx); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(ctx context.Context) error {
	if *buildYamlPath == "" {
		return fmt.Errorf("--build-yaml flag is required")
	}
	if *outputDir == "" {
		return fmt.Errorf("--output-dir flag is required")
	}

	buildInfo, err := loadBuildYaml(*buildYamlPath)
	if err != nil {
		return fmt.Errorf("loading build yaml: %w", err)
	}

	chartsBranch := buildInfo.getRancherChartsBranch()
	if chartsBranch == "" {
		return fmt.Errorf("couldn't find charts branch in build.yaml")
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	index, err := loadRancherChartsIndex(ctx, chartsBranch)
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	toDownload, err := entriesToDownload(index, buildInfo.getSystemCharts())
	if err != nil {
		return fmt.Errorf("processing charts to download: %w", err)
	}

	cacheDir, err := os.MkdirTemp("", "charts-cache")
	if err != nil {
		return fmt.Errorf("creating temporary directory: %w", err)
	}
	defer os.RemoveAll(cacheDir)
	chartDownloader := downloader.ChartDownloader{
		Out:          os.Stderr,
		Options:      []getter.Option{getter.WithTimeout(60 * time.Second)},
		Getters:      getter.Getters(),
		ContentCache: cacheDir,
	}

	for _, chart := range toDownload {
		if err := downloadChart(chartDownloader, chartsBranch, chart.URLs[0], *outputDir); err != nil {
			return fmt.Errorf("downloading chart %s-%s: %w", chart.Name, chart.Version, err)
		}
	}

	return nil
}

// entriesToDownload resolves requested charts and their auto-install dependencies against index.yaml, skipping duplicates.
func entriesToDownload(index *repo.IndexFile, charts map[string]string) ([]*repo.ChartVersion, error) {
	return entriesToDownloadWithSeen(index, charts, make(map[string]bool))
}

func entriesToDownloadWithSeen(index *repo.IndexFile, charts map[string]string, seen map[string]bool) ([]*repo.ChartVersion, error) {
	res := make([]*repo.ChartVersion, 0, len(charts))
	deps := make(map[string]string)
	for name, version := range charts {
		if version == "" {
			return nil, fmt.Errorf("couldn't find version for chart %q in build.yaml", name)
		}
		key := name + "@" + version
		if seen[key] {
			continue
		}
		var found bool
		for _, entry := range index.Entries[name] {
			if entry.Version != version || len(entry.URLs) == 0 {
				continue
			}
			found = true
			seen[key] = true
			res = append(res, entry)

			// Parse auto-install annotation (format: "<depName>=<depVersion|match>").
			if autoInstall := entry.Annotations["catalog.cattle.io/auto-install"]; autoInstall != "" {
				sp := strings.SplitN(autoInstall, "=", 2)
				depName, depVersion := sp[0], version
				if len(sp) == 2 && sp[1] != "match" {
					depVersion = sp[1]
				}
				deps[depName] = depVersion
			}
			break
		}
		if !found {
			return nil, fmt.Errorf("couldn't find version for chart %q in index.yaml", name)
		}
	}

	// Recursively resolve and append dependent charts.
	if len(deps) > 0 {
		depEntries, err := entriesToDownloadWithSeen(index, deps, seen)
		if err != nil {
			return nil, err
		}
		res = append(res, depEntries...)
	}

	return res, nil
}

// loadBuildYaml reads build.yaml into a map of configuration key-value pairs.
func loadBuildYaml(p string) (buildYaml, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	var data buildYaml
	if err := yaml.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

type buildYaml map[string]string

func (d buildYaml) getRancherChartsBranch() string {
	return d["chartDefaultBranch"]
}

func (d buildYaml) getSystemCharts() map[string]string {
	return map[string]string{
		"fleet":              d["fleetVersion"],
		"rancher-turtles":    d["turtlesVersion"],
		"rancher-webhook":    d["webhookVersion"],
		"remotedialer-proxy": d["remoteDialerProxyVersion"],
	}

}

const rancherChartsRepoRoot = "https://raw.githubusercontent.com/rancher/charts/"

// loadRancherChartsIndex fetches and decodes index.yaml for the specified charts repository branch.
func loadRancherChartsIndex(ctx context.Context, branch string) (*repo.IndexFile, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	indexURL := rancherChartsRepoRoot + branch + "/index.yaml"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response fetching %s: %s", indexURL, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data repo.IndexFile
	if err := yaml.UnmarshalStrict(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// downloadChart retrieves a tarball from its URL, using a "helm pull" equivalent
func downloadChart(chartDownloader downloader.ChartDownloader, branch, url, dest string) error {
	if strings.HasPrefix(url, "oci://") || strings.HasPrefix(url, "https://") {
		// absolute URL
	} else {
		// relative path, build final URL
		url = rancherChartsRepoRoot + branch + "/" + url
	}

	p, _, err := chartDownloader.DownloadTo(url, "", dest)
	if err != nil {
		return err
	}

	fmt.Printf("chart downloaded to %s...\n", p)
	return nil
}
