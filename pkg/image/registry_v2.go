package image

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
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
	if again, err := retry.DefaultPredicate(resp, err); again || err != nil {
		return again, err
	}
	return resp != nil && resp.StatusCode >= http.StatusInternalServerError, nil
}

// ChartImage is an image of the list that comes from charts, with the charts it comes from, as name:version.
type ChartImage struct {
	Image   string
	Sources []string
}

// ImagesOfCharts returns the images of the list, in the "<image> <source>,<source>" lines that GetArtifacts gives, that
// come from at least one chart, with only the sources that are charts. The source of a chart is its name and version,
// with a colon, and the one of any other image is a word like rancher or rke2All. The lines of an image that is in
// more than one are merged.
func ImagesOfCharts(imagesAndSources []string) []ChartImage {
	var order []string
	sources := map[string]map[string]struct{}{}
	for _, line := range imagesAndSources {
		image, lineSources, _ := strings.Cut(line, " ")
		for _, source := range strings.Split(lineSources, ",") {
			if !strings.Contains(source, ":") {
				continue
			}
			if sources[image] == nil {
				sources[image] = map[string]struct{}{}
				order = append(order, image)
			}
			sources[image][source] = struct{}{}
		}
	}

	images := make([]ChartImage, 0, len(order))
	for _, image := range order {
		charts := make([]string, 0, len(sources[image]))
		for source := range sources[image] {
			charts = append(charts, source)
		}
		sort.Strings(charts)
		images = append(images, ChartImage{Image: image, Sources: charts})
	}
	return images
}

// RegistryOfAssetsImage returns the registry that the images of the charts are in for a build with that Rancher Assets
// image: the staging one when the tag of the image ends with -dev, and the production one otherwise.
func RegistryOfAssetsImage(assetsImage string) string {
	tag := assetsImage[strings.LastIndex(assetsImage, ":")+1:]
	if strings.HasSuffix(tag, "-dev") {
		return stagingImagesRegistry
	}
	return productionImagesRegistry
}

// CheckImagesExist asks the registry, without credentials, whether each image is in it with its tag, and returns an
// error that names the images that are not, or that it could not ask for, with their charts.
func CheckImagesExist(registry string, images []ChartImage) error {
	client := &auth.Client{
		Client: &http.Client{Transport: &retry.Transport{Policy: registryRetryPolicy}},
		Cache:  auth.NewCache(),
	}

	sorted := append([]ChartImage(nil), images...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Image < sorted[j].Image })

	var problems []string
	for _, image := range sorted {
		if problem := checkImageExists(client, registry, image); problem != "" {
			problems = append(problems, problem)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s does not have these images of the charts, or did not answer for them:\n%s", registry, strings.Join(problems, "\n"))
}

// checkImageExists asks the registry for the manifest of the image, and returns what is wrong with it, or an empty
// string when it is there. An image that the registry does not have is told from one that it could not be asked for.
func checkImageExists(client *auth.Client, registry string, image ChartImage) string {
	description := fmt.Sprintf("  %s (%s)", image.Image, strings.Join(image.Sources, ", "))

	separator := strings.LastIndex(image.Image, ":")
	if separator < 0 || strings.Contains(image.Image[separator:], "/") {
		return description + ": has no tag"
	}
	repository, err := remote.NewRepository(registry + "/" + image.Image[:separator])
	if err != nil {
		return fmt.Sprintf("%s: %v", description, err)
	}
	repository.PlainHTTP = registryPlainHTTP
	repository.Client = client

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := repository.Resolve(ctx, image.Image[separator+1:]); err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return fmt.Sprintf("%s: is not in %s", description, registry)
		}
		return fmt.Sprintf("%s: could not be asked at %s: %v", description, registry, err)
	}
	return ""
}
