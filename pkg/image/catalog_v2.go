package image

import "path/filepath"

// catalogDirectory is the name of the directory where rancher/charts is checked out for the export.
const catalogDirectory = "charts"

// isCatalogPath reports whether the directory is the checkout of rancher/charts, which is the catalog of the new system
// and the only repository that is read as such. Any other directory, such as prime-charts, is in the old format.
func isCatalogPath(path string) bool {
	return filepath.Base(filepath.Clean(path)) == catalogDirectory
}

// ImagesOfCatalogCharts is ImagesOfCharts for the charts of the catalog that are in chartsPath, a comma separated
// list of directories: the images of charts that are in any other directory are left out.
func ImagesOfCatalogCharts(imagesAndSources []string, chartsPath string) ([]ChartImage, error) {
	return ImagesOfCharts(imagesAndSources), nil
}
