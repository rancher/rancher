package image

import (
	"path/filepath"
	"strings"

	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// catalogDirectory is the name of the directory where rancher/charts is checked out for the export.
const catalogDirectory = "charts"

// isCatalogPath reports whether the directory is the checkout of rancher/charts, which is the catalog of the new system
// and the only repository that is read as such. Any other directory, such as prime-charts, is in the old format.
func isCatalogPath(path string) bool {
	return filepath.Base(filepath.Clean(path)) == catalogDirectory
}

// ImagesOfCatalogCharts is ImagesOfCharts for the charts of the catalog that are in chartsPath, a comma separated
// list of directories: a source that is a chart of any other directory is left out, and so is an image that has no
// other. A chart is the catalog's when its name and version are in the index.yaml of the catalog.
func ImagesOfCatalogCharts(imagesAndSources []string, chartsPath string) ([]ChartImage, error) {
	catalog := map[string]struct{}{}
	for _, path := range strings.Split(chartsPath, ",") {
		if !isCatalogPath(path) {
			continue
		}
		index, err := repo.LoadIndexFile(filepath.Join(path, "index.yaml"))
		if err != nil {
			return nil, err
		}
		for name, versions := range index.Entries {
			for _, version := range versions {
				catalog[name+":"+version.Version] = struct{}{}
			}
		}
	}

	var images []ChartImage
	for _, image := range ImagesOfCharts(imagesAndSources) {
		var sources []string
		for _, source := range image.Sources {
			if _, ok := catalog[source]; ok {
				sources = append(sources, source)
			}
		}
		if len(sources) > 0 {
			images = append(images, ChartImage{Image: image.Image, Sources: sources})
		}
	}
	return images, nil
}
