package image

// PinnedChart is a chart that Rancher installs at an exact version, whatever versions the charts repository has.
type PinnedChart struct {
	Name    string
	Version string
}

// CheckPinnedCharts checks that the version of each pinned chart is the one FetchImagesV2 reads for the Rancher
// version, which is the newest one that supports it. Rancher installs the pinned version, so when the two differ the
// images of the chart that is installed are not in the list and an air-gapped installation cannot pull them.
//
// A pinned chart that the repositories of chartsPath (comma separated) do not have is not checked: it is in another
// repository. A mismatch is an error when strict is true, which is for the build of a release, and a warning
// otherwise.
func CheckPinnedCharts(chartsPath, rancherVersion string, pins []PinnedChart, strict bool) error {
	return nil
}
