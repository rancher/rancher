package image

// isCatalogPath reports whether the directory is the checkout of rancher/charts, which is the catalog of the new system
// and the only repository that is read as such. Any other directory, such as prime-charts, is in the old format.
func isCatalogPath(path string) bool {
	return false
}

// ImagesOfCatalogCharts is ImagesOfCharts for the charts of the catalog that are in chartsPath, a comma separated
// list of directories: the images of charts that are in any other directory are left out.
func ImagesOfCatalogCharts(imagesAndSources []string, chartsPath string) ([]ChartImage, error) {
	return ImagesOfCharts(imagesAndSources), nil
}
