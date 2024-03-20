package image

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	util "github.com/rancher/rancher/pkg/cluster"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	img "github.com/rancher/rke/types/image"
)

// ExportConfig provides parameters you can define to configure image exporting for Rancher components
type ExportConfig struct {
	RancherVersion   string
	OsType           OSType
	ChartsPath       string
	SystemChartsPath string
	GithubEndpoints  []GithubEndpoint
}

type OSType int

const (
	Linux OSType = iota
	Windows
)

const imageListDelimiter = "\n"

var osTypeImageListName = map[OSType]string{
	Windows: "windows-rancher-images",
	Linux:   "rancher-images",
}

// Resolve calls ResolveWithCluster passing nil into the cluster argument.
// returns the image concatenated with the URL of the system default registry.
// if there is no system default registry it will return the image
func Resolve(image string) string {
	return ResolveWithCluster(image, nil)
}

// ResolveWithCluster returns the image concatenated with the URL of the private registry specified, adding rancher/ if is a private repo.
// It will use the cluster level registry if one is found, or the system default registry if no cluster level registry is found.
// If either is not found, it returns the image.
func ResolveWithCluster(image string, cluster *v3.Cluster) string {
	reg := util.GetPrivateRegistryURL(cluster)
	if reg != "" && !strings.HasPrefix(image, reg) {
		// Images from Dockerhub Library repo, we add rancher prefix when using private registry
		if !strings.Contains(image, "/") {
			image = "rancher/" + image
		}
		return path.Join(reg, image)
	}

	return image
}

// GetImages fetches the list of container images used in the sources provided in the exportConfig.
// Rancher charts, system charts, system images and extension images of Rancher are fetched.
// GetImages is called during runtime by Rancher catalog package which is deprecated.
// It is actually used for generation rancher-images.txt for airgap scenarios.
func GetImages(exportConfig ExportConfig, externalImages map[string][]string, imagesFromArgs []string, rkeSystemImages map[string]rketypes.RKESystemImages) ([]string, []string, error) {
	imagesSet := make(map[string]map[string]struct{})

	// fetch images from charts
	charts := Charts{exportConfig}
	if err := charts.FetchImages(imagesSet); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch images from charts")
	}

	// fetch images from system charts
	systemCharts := SystemCharts{exportConfig}
	if err := systemCharts.FetchImages(imagesSet); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch images from system charts")
	}

	// fetch images from system images
	system := System{exportConfig}
	if err := system.FetchImages(rkeSystemImages, imagesSet); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch images from system")
	}

	// fetch images from extension catalog images
	extensions := ExtensionsConfig{exportConfig}
	if err := extensions.FetchExtensionImages(imagesSet); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch images from extensions")
	}

	setRequirementImages(exportConfig.OsType, imagesSet)

	// set rancher images from args
	setImages("rancher", imagesFromArgs, imagesSet)

	for source, sourceImages := range externalImages {
		setImages(source, sourceImages, imagesSet)
	}

	convertMirroredImages(imagesSet)

	imagesList, imagesAndSourcesList := generateImageAndSourceLists(imagesSet)

	return imagesList, imagesAndSourcesList, nil
}

func AddImagesToImageListConfigMap(cm *v1.ConfigMap, rancherVersion, systemChartsPath string) error {
	exportConfig := ExportConfig{
		SystemChartsPath: systemChartsPath,
		OsType:           Windows,
		RancherVersion:   rancherVersion,
	}
	windowsImages, _, err := GetImages(exportConfig, nil, []string{}, nil)
	if err != nil {
		return err
	}
	exportConfig.OsType = Linux
	linuxImages, _, err := GetImages(exportConfig, nil, []string{}, nil)
	if err != nil {
		return err
	}
	cm.Data = make(map[string]string, 2)
	cm.Data[osTypeImageListName[Windows]] = strings.Join(windowsImages, imageListDelimiter)
	cm.Data[osTypeImageListName[Linux]] = strings.Join(linuxImages, imageListDelimiter)
	return nil
}

func ParseCatalogImageListConfigMap(cm *v1.ConfigMap) ([]string, []string) {
	windowsImages := strings.Split(cm.Data[osTypeImageListName[Windows]], imageListDelimiter)
	linuxImages := strings.Split(cm.Data[osTypeImageListName[Linux]], imageListDelimiter)
	return windowsImages, linuxImages
}

func IsValidSemver(version string) bool {
	_, err := semver.NewVersion(version)
	return err == nil
}

func setRequirementImages(osType OSType, imagesSet map[string]map[string]struct{}) {
	coreLabel := "core"
	switch osType {
	case Linux:
		addSourceToImage(imagesSet, settings.ShellImage.Get(), coreLabel)
		addSourceToImage(imagesSet, settings.MachineProvisionImage.Get(), coreLabel)
		addSourceToImage(imagesSet, "rancher/mirrored-bci-busybox:15.4.11.2", coreLabel)
		addSourceToImage(imagesSet, "rancher/mirrored-bci-micro:15.4.14.3", coreLabel)
	}
}

func setImages(source string, imagesFromArgs []string, imagesSet map[string]map[string]struct{}) {
	for _, image := range imagesFromArgs {
		addSourceToImage(imagesSet, image, source)
	}
}

func addSourceToImage(imagesSet map[string]map[string]struct{}, image string, sources ...string) {
	if image == "" {
		return
	}
	if imagesSet[image] == nil {
		imagesSet[image] = make(map[string]struct{})
	}
	for _, source := range sources {
		imagesSet[image][source] = struct{}{}
	}
}

func convertMirroredImages(imagesSet map[string]map[string]struct{}) {
	for image := range imagesSet {
		convertedImage := img.Mirror(image)
		if image == convertedImage {
			continue
		}
		for source := range imagesSet[image] {
			addSourceToImage(imagesSet, convertedImage, source)
		}
		delete(imagesSet, image)
	}
}

func generateImageAndSourceLists(imagesSet map[string]map[string]struct{}) ([]string, []string) {
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

func getSourcesList(imageSources map[string]struct{}) string {
	var sources []string

	for source := range imageSources {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return strings.Join(sources, ",")
}
