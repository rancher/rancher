package image

import (
	"net/http"
	"time"

	"oras.land/oras-go/v2/registry/remote/retry"
)

// The registries that the images of the charts are in: staging for a development build and production for a release.
const (
	stagingImagesRegistry    = "stgregistry.suse.com"
	productionImagesRegistry = "registry.suse.com"
)

// registryAttempts is how many times a registry is asked for an image before giving up.
const registryAttempts = 5

// registryRetryPolicy is the policy of the requests to a registry: the wait between the attempts, and their number.
var registryRetryPolicy = func() retry.Policy {
	return &retry.GenericPolicy{
		Retryable: retryable,
		Backoff:   retry.ExponentialBackoff(time.Second, 2, 0.1),
		MinWait:   time.Second,
		MaxWait:   10 * time.Second,
		MaxRetry:  registryAttempts - 1,
	}
}

// registryPlainHTTP makes the requests to a registry go without TLS, which is for the registry of a test.
var registryPlainHTTP = false

// retryable reports whether a request that failed has to be made again: when the registry did not answer, or said that
// it failed or that it is too busy.
func retryable(resp *http.Response, err error) (bool, error) {
	return false, nil
}

// ChartImage is an image of the list that comes from charts, with the charts it comes from, as name:version.
type ChartImage struct {
	Image   string
	Sources []string
}

// ImagesOfCharts returns the images of the list, in the "<image> <source>,<source>" lines that GetArtifacts gives, that
// come from at least one chart, with only the sources that are charts. The source of a chart is its name and version,
// with a colon, and the one of any other image is a word like rancher or rke2All.
func ImagesOfCharts(imagesAndSources []string) []ChartImage {
	return nil
}

// RegistryOfAssetsImage returns the registry that the images of the charts are in for a build with that Rancher Assets
// image: the staging one when the tag of the image ends with -dev, and the production one otherwise.
func RegistryOfAssetsImage(assetsImage string) string {
	return ""
}

// CheckImagesExist asks the registry, without credentials, whether each image is in it with its tag, and returns an
// error that names the images that are not, or that it could not ask for, with their charts.
func CheckImagesExist(registry string, images []ChartImage) error {
	return nil
}
