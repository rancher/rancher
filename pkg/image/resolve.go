package image

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"
	util "github.com/rancher/rancher/pkg/cluster"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	img "github.com/rancher/rke/types/image"
)

type OSType int

const imageListDelimiter = "\n"
const (
	Linux OSType = iota
	Windows
)

var osTypeImageListName = map[OSType]string{
	Windows: "windows-rancher-images",
	Linux:   "rancher-images",
}

func Resolve(image string) string {
	return ResolveWithCluster(image, nil)
}

func ResolveWithCluster(image string, cluster *v3.Cluster) string {
	reg := util.GetPrivateRepoURL(cluster)
	if reg != "" && !strings.HasPrefix(image, reg) {
		//Images from Dockerhub Library repo, we add rancher prefix when using private registry
		if !strings.Contains(image, "/") {
			image = "rancher/" + image
		}
		return path.Join(reg, image)
	}

	return image
}

func addSourceToImage(imagesSet map[string]map[string]bool, image string, sources ...string) {
	if image == "" {
		return
	}
	if imagesSet[image] == nil {
		imagesSet[image] = make(map[string]bool)
	}
	for _, source := range sources {
		imagesSet[image][source] = true
	}
}

func GetImages(systemChartPath, chartPath string, externalImages map[string][]string, imagesFromArgs []string, rkeSystemImages map[string]rketypes.RKESystemImages, osType OSType) ([]string, []string, error) {
	// fetch images from system charts
	imagesSet := make(map[string]map[string]bool)
	if systemChartPath != "" {
		if err := fetchImagesFromCharts(systemChartPath, osType, imagesSet); err != nil {
			return nil, nil, errors.Wrap(err, "failed to fetch images from system charts")
		}
	}

	// fetch images from charts
	if chartPath != "" {
		if err := fetchImagesFromCharts(chartPath, osType, imagesSet); err != nil {
			return nil, nil, errors.Wrap(err, "failed to fetch images from charts")
		}
	}

	// fetch images from system images
	if len(rkeSystemImages) > 0 {
		if err := fetchImagesFromSystem(rkeSystemImages, osType, imagesSet); err != nil {
			return nil, nil, errors.Wrap(err, "failed to fetch images from system images")
		}
	}

	setRequirementImages(osType, imagesSet)

	// set rancher images from args
	setImages("rancher", imagesFromArgs, imagesSet)

	for source, sourceImages := range externalImages {
		setImages(source, sourceImages, imagesSet)
	}

	convertMirroredImages(imagesSet)

	imagesList, imagesAndSourcesList := generateImageAndSourceLists(imagesSet)

	return imagesList, imagesAndSourcesList, nil
}

func setImages(source string, imagesFromArgs []string, imagesSet map[string]map[string]bool) {
	for _, image := range imagesFromArgs {
		addSourceToImage(imagesSet, image, source)
	}
}

func convertMirroredImages(imagesSet map[string]map[string]bool) {
	for image := range imagesSet {
		convertedImage := img.Mirror(image)
		if image == convertedImage {
			continue
		}
		for source, val := range imagesSet[image] {
			if !val {
				continue
			}
			addSourceToImage(imagesSet, convertedImage, source)
		}
		delete(imagesSet, image)
	}
}

func setRequirementImages(osType OSType, imagesSet map[string]map[string]bool) {
	coreLabel := "core"
	switch osType {
	case Linux:
		addSourceToImage(imagesSet, settings.ShellImage.Get(), coreLabel)
		addSourceToImage(imagesSet, settings.MachineProvisionImage.Get(), coreLabel)
		addSourceToImage(imagesSet, "busybox", coreLabel)
	}
}

func generateImageAndSourceLists(imagesSet map[string]map[string]bool) ([]string, []string) {
	var images, imagesAndSources []string
	// unique
	for image := range imagesSet {
		images = append(images, image)
	}

	// sort
	sort.Slice(images, func(i, j int) bool {
		return images[i] < images[j]
	})

	for _, image := range images {
		imagesAndSources = append(imagesAndSources, fmt.Sprintf("%s %s", image, getSourcesList(imagesSet[image])))
	}

	return images, imagesAndSources
}

func getSourcesList(imageSources map[string]bool) string {
	var sources []string

	for source, val := range imageSources {
		if !val {
			continue
		}
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return strings.Join(sources, ",")
}

func AddImagesToImageListConfigMap(cm *v1.ConfigMap, chartPath string) (err error) {
	var windowsImages []string
	var linuxImages []string

	windowsImages, _, err = GetImages(chartPath, "", nil, []string{}, nil, Windows)
	if err != nil {
		return
	}

	linuxImages, _, err = GetImages(chartPath, "", nil, []string{}, nil, Linux)
	if err != nil {
		return
	}

	cm.Data = make(map[string]string, 2)
	cm.Data[osTypeImageListName[Windows]] = strings.Join(windowsImages, imageListDelimiter)
	cm.Data[osTypeImageListName[Linux]] = strings.Join(linuxImages, imageListDelimiter)

	return
}

func ParseCatalogImageListConfigMap(cm *v1.ConfigMap) (windowsImageList, linuxImageList []string) {
	windowsImageList = strings.Split(cm.Data[osTypeImageListName[Windows]], imageListDelimiter)
	linuxImageList = strings.Split(cm.Data[osTypeImageListName[Linux]], imageListDelimiter)

	return
}
