package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/image/utilities"

	img "github.com/rancher/rancher/pkg/image"
)

// This file attempts to create an updated mapping of images used in rancher and their source code origin.
// It does so on a best-effort basis and prints the resulting map to the terminal. Rancher images which share a name
// with their base repository (such as gke-operator, which comes from github.com/rancher/gke-operator) will be
// automatically resolved. In the case where images are detected but cannot be automatically resolved, a warning
// will be printed to the console indicating such images, and the value of the image in the map will be 'unknown'.
//
// This tool can be run with the command `dapper check-origins`.

func main() {
	if err := inner(os.Args[1], os.Args[2], os.Args[3:]); err != nil {
		panic(err)
	}
}

const imageNotFound = "image not found"

func inner(systemChartsPath, chartsPath string, imagesFromArgs []string) error {
	targetsAndSources, err := utilities.GatherTargetImagesAndSources(systemChartsPath, chartsPath, imagesFromArgs)
	if err != nil {
		return err
	}

	unusedImages := CheckForImagesNoLongerBeingUsed(targetsAndSources)
	if len(unusedImages) > 0 {
		fmt.Println("Some images are no longer used by Rancher, please remove the following images from pkg/image/origin.go: ", unusedImages)
	}

	return PrintUpdatedImageOrigins(targetsAndSources)
}

// CheckForImagesNoLongerBeingUsed determines if /pkg/img/origins.go has keys within the map which
// are no longer relevant. If so, they should be removed from /pkg/img/origins.go so that rancher-origins.txt
// is up-to-date for the current version of Rancher.
func CheckForImagesNoLongerBeingUsed(targetsAndSources utilities.ImageTargetsAndSources) []string {
	currentImages := make(map[string]interface{})
	for _, e := range img.UniqueTargetImages(targetsAndSources.LinuxImagesFromArgs) {
		currentImages[e] = true
	}

	for _, e := range img.UniqueTargetImages(targetsAndSources.TargetLinuxImages) {
		currentImages[e] = true
	}

	for _, e := range img.UniqueTargetImages(targetsAndSources.TargetWindowsImages) {
		currentImages[e] = true
	}

	var unusedImages []string
	for k := range img.OriginMap {
		_, ok := currentImages[k]
		if !ok {
			unusedImages = append(unusedImages, k)
		}
	}

	return unusedImages
}

func PrintUpdatedImageOrigins(targetsAndSources utilities.ImageTargetsAndSources) error {
	fmt.Println("Generating updated rancher-image-origins map")

	// use the existing map so that we don't
	// check images which have already been resolved.
	imgToURL := img.OriginMap

	// look through any images passed as arguments
	err := convertImagesToRepoUrls(targetsAndSources.LinuxImagesFromArgs, imgToURL)
	if err != nil {
		return err
	}
	// look through the linux target images
	err = convertImagesToRepoUrls(targetsAndSources.TargetLinuxImages, imgToURL)
	if err != nil {
		return err
	}
	// look through the windows target images
	err = convertImagesToRepoUrls(targetsAndSources.TargetWindowsImages, imgToURL)
	if err != nil {
		return err
	}

	// order images alphabetically
	type sorter struct {
		img string
		url string
	}
	var sorted []sorter
	for k, v := range imgToURL {
		sorted = append(sorted, sorter{img: k, url: v})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].img < sorted[j].img
	})

	x := "OriginMap = map[string]string{\n"
	for _, e := range sorted {
		x = x + "	\"" + e.img + "\":" + " " + "\"" + e.url + "\"," + "\n"
	}
	x = x + "}\n"

	// print result, can be copied into pkg/image/origins.go
	// to automatically update some images.
	fmt.Println(x)

	// find all images that have not been resolved yet
	var unknownImages string
	for k, v := range imgToURL {
		if v == "" || v == "unknown" {
			unknownImages = unknownImages + "\n" + k
		}
	}

	// warn about unresolved images
	// so that they may be manually resolved
	if len(unknownImages) > 0 {
		fmt.Println(fmt.Sprintf("[WARN] Some images do not have an origin defined, please provide origins within rancher/pkg/image/origins.go for the following images: %s", unknownImages))
	}

	return nil
}

// convertImagesToRepoUrls attempts to resolve Rancher owned images with its origin repository.
// It does so on a best-effort basis, any gaps left by this must be covered manually
// in the OriginMap in origins.go
func convertImagesToRepoUrls(images []string, imgToURL map[string]string) error {
	for _, repo := range img.UniqueTargetImages(images) {
		if v, ok := imgToURL[repo]; ok && (v != "" && v != "unknown") {
			continue // image is already resolved
		}
		switch {
		case strings.Contains(repo, "mirrored"):
			// mirrored images, assumes everything after 'mirrored-'
			// can be directly converted into a github repository.
			ownerAndRepository := strings.Split(strings.ReplaceAll(repo, "mirrored-", ""), "-")
			if len(ownerAndRepository) <= 1 {
				imgToURL[repo] = "unknown"
				continue
			}
			url, err := checkURL(fmt.Sprintf("https://github.com/%s/%s", ownerAndRepository[0], strings.Join(ownerAndRepository[1:], "")))
			if err != nil && err.Error() != imageNotFound {
				return fmt.Errorf("encountered HTTP error while resolving repository %s: %w", repo, err)
			}
			if err != nil {
				url = "unknown"
			}
			imgToURL[repo] = url
		case strings.Contains(repo, "hardened"):
			// hardened images conversion, best effort
			cleanRepo := strings.ReplaceAll(repo, "hardened-", "image-build-")
			url, err := checkURL("https://github.com/rancher/" + cleanRepo)
			if err != nil && err.Error() != imageNotFound {
				return fmt.Errorf("encountered HTTP error while resolving repository %s: %w", repo, err)
			}
			if err != nil {
				url = "unknown"
			}
			imgToURL[repo] = url
		default:
			// handle Rancher images which share
			// a name with their base repository
			url, err := checkURL("https://github.com/rancher/" + repo)
			if err != nil && err.Error() != imageNotFound {
				return fmt.Errorf("encountered HTTP error while resolving repository %s: %w", repo, err)
			}
			if err != nil {
				url = "unknown"
			}
			imgToURL[repo] = url
		}
	}
	return nil
}

// checkURL performs a GET request against a github repository
// to determine if it exists or not.
func checkURL(url string) (string, error) {
	r, err := http.Get(url)
	if err != nil {
		return "", err
	}
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf(imageNotFound)
	}
	return url, nil
}
