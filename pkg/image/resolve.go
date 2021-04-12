package image

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	libhelm "github.com/rancher/rancher/pkg/catalog/helm"
	util "github.com/rancher/rancher/pkg/cluster"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	img "github.com/rancher/rke/types/image"
	"gopkg.in/yaml.v2"
)

type OSType int

const (
	Linux OSType = iota
	Windows
	SystemChartsRepoDir = "build/system-charts"
)

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

func getChartVersions(path, rancherVersion string) (libhelm.ChartVersions, error) {
	chartVersions := libhelm.ChartVersions{}
	helm := libhelm.Helm{
		LocalPath: path,
		IconPath:  path,
		Hash:      "",
	}
	index, err := helm.LoadIndex()
	if err != nil {
		return nil, err
	}
	for _, versions := range index.IndexFile.Entries {
		if len(versions) > 0 {
			// Add latest version (First element is latest, list sorted in descending order)
			chartVersions = append(chartVersions, versions[0])
			if strings.Contains(path, SystemChartsRepoDir) {
				// Skip if rancherVersion is not a valid semver (E.g is a commit hash)
				if rancherSemVer, err := semver.NewVersion(rancherVersion); err == nil {
					// Skip first element, the latest version has already been added
					for _, v := range versions[1:] {
						questions, err := getQuestions(filepath.Join(path, v.Dir))
						if err != nil {
							return nil, fmt.Errorf("no questions file in chart %s:%s", v.Name, v.Version)
						}
						min, ok := questions["rancher_min_version"].(string)
						if !ok {
							return nil, fmt.Errorf("no rancher_min_version set in chart %s:%s", v.Name, v.Version)
						}
						// No ok check, max is optional
						max, _ := questions["rancher_max_version"].(string)
						isInRange, err := isInMinMaxRange(rancherSemVer, min, max)
						if err != nil {
							return nil, err
						}
						if isInRange {
							chartVersions = append(chartVersions, v)
						}
					}
				}
			}
		}
	}
	return chartVersions, nil
}

func getQuestions(versionPath string) (map[interface{}]interface{}, error) {
	content, err := ioutil.ReadFile(filepath.Join(versionPath, "questions.yaml"))
	if err != nil {
		content, err = ioutil.ReadFile(filepath.Join(versionPath, "questions.yml"))
		if err != nil {
			return nil, err
		}
	}
	questions := make(map[interface{}]interface{})
	if err = yaml.Unmarshal(content, &questions); err != nil {
		return nil, err
	}
	return questions, nil
}

func isInMinMaxRange(rancherSemVer *semver.Version, min, max string) (bool, error) {
	var minSemVer, maxSemVer *semver.Version
	var isInRange bool
	var err error
	if minSemVer, err = semver.NewVersion(strings.TrimSpace(min)); err == nil {
		if maxSemVer, err = semver.NewVersion(strings.TrimSpace(max)); err == nil {
			isInRange = (rancherSemVer.GreaterThan(minSemVer) || rancherSemVer.Equal(minSemVer)) &&
				(rancherSemVer.LessThan(maxSemVer) || rancherSemVer.Equal(maxSemVer))
			return isInRange, nil
		}
		isInRange = rancherSemVer.GreaterThan(minSemVer) || rancherSemVer.Equal(minSemVer)
		return isInRange, nil
	}
	return false, err
}

func pickImagesFromValuesYAML(imagesSet map[string]map[string]bool, chartVersions libhelm.ChartVersions, basePath, path string, info os.FileInfo, osType OSType) error {
	if info.Name() != "values.yaml" {
		return nil
	}
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return err
	}
	var chartNameAndVersion string
	for _, v := range chartVersions {
		if strings.HasPrefix(relPath, v.Dir) {
			chartNameAndVersion = fmt.Sprintf("%s:%s", v.Name, v.Version)
			break
		}
	}
	if chartNameAndVersion == "" {
		// path does not belong to a given chart
		return nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	chartValues := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(data, &chartValues); err != nil {
		return err
	}

	var imagesFound []string
	walkthroughMap(chartValues, func(inputMap map[interface{}]interface{}) {
		generateImages(chartNameAndVersion, inputMap, imagesSet, &imagesFound, osType)
	})
	log.Printf("Images found in %s: %+v", path, imagesFound)
	return nil
}

func generateImages(chartNameAndVersion string, inputMap map[interface{}]interface{}, output map[string]map[string]bool, imagesFound *[]string, osType OSType) {
	repo, ok := inputMap["repository"].(string)
	if !ok {
		return
	}
	tag, ok := inputMap["tag"]
	if !ok {
		return
	}

	imageName := fmt.Sprintf("%s:%v", repo, tag)
	*imagesFound = append(*imagesFound, imageName)

	// By default, images are added to the generic images list ("linux"). For Windows and multi-OS
	// images to be considered, they must use a comma-delineated list (e.g. "os: windows",
	// "os: windows,linux", and "os: linux,windows").
	if osList, ok := inputMap["os"].(string); ok {
		for _, os := range strings.Split(osList, ",") {
			switch strings.TrimSpace(strings.ToLower(os)) {
			case "windows":
				if osType == Windows {
					addSourceToImage(output, imageName, chartNameAndVersion)
					return
				}
			case "linux":
				if osType == Linux {
					addSourceToImage(output, imageName, chartNameAndVersion)
					return
				}
			}
		}
	} else {
		if inputMap["os"] != nil {
			panic(fmt.Sprintf("Field 'os:' for image %s contains neither a string nor nil", imageName))
		}
		if osType == Linux {
			addSourceToImage(output, imageName, chartNameAndVersion)
		}
	}
}

func addSourceToImage(imagesSet map[string]map[string]bool, image string, sources ...string) {
	if imagesSet[image] == nil {
		imagesSet[image] = make(map[string]bool)
	}
	for _, source := range sources {
		imagesSet[image][source] = true
	}
}

func walkthroughMap(inputMap map[interface{}]interface{}, walkFunc func(map[interface{}]interface{})) {
	walkFunc(inputMap)
	for _, value := range inputMap {
		if v, ok := value.(map[interface{}]interface{}); ok {
			walkthroughMap(v, walkFunc)
		}
	}
}

func GetImages(systemChartPath, chartPath, rancherVersion string, k3sUpgradeImages, imagesFromArgs []string, rkeSystemImages map[string]rketypes.RKESystemImages, osType OSType) ([]string, []string, error) {
	// fetch images from system charts
	imagesSet := make(map[string]map[string]bool)
	if systemChartPath != "" {
		if err := fetchImagesFromCharts(systemChartPath, rancherVersion, osType, imagesSet); err != nil {
			return nil, nil, errors.Wrap(err, "failed to fetch images from system charts")
		}
	}

	// fetch images from charts
	if chartPath != "" {
		if err := fetchImagesFromCharts(chartPath, rancherVersion, osType, imagesSet); err != nil {
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

	// set images for k3s-upgrade
	setImages("k3sUpgrade", k3sUpgradeImages, imagesSet)

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

func fetchImagesFromCharts(path, rancherVersion string, osType OSType, imagesSet map[string]map[string]bool) error {
	chartVersions, err := getChartVersions(path, rancherVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart and version from %q", path)
	}

	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return pickImagesFromValuesYAML(imagesSet, chartVersions, path, p, info, osType)
	})
	if err != nil {
		return errors.Wrap(err, "failed to pick images from values.yaml")
	}

	return nil
}

func fetchImagesFromSystem(rkeSystemImages map[string]rketypes.RKESystemImages, osType OSType, imagesSet map[string]map[string]bool) error {
	collectionImagesList := []interface{}{
		rkeSystemImages,
	}
	switch osType {
	case Linux:
		collectionImagesList = append(collectionImagesList, v32.ToolsSystemImages)
	}

	images, err := flatImagesFromCollections(collectionImagesList...)
	if err != nil {
		return err
	}

	for _, image := range images {
		addSourceToImage(imagesSet, image, "system")

	}
	return nil
}

func flatImagesFromCollections(cols ...interface{}) (images []string, err error) {
	for _, col := range cols {
		colObj := map[string]interface{}{}
		if err := convert.ToObj(col, &colObj); err != nil {
			return []string{}, err
		}

		images = append(images, fetchImagesFromCollection(colObj)...)
	}
	return images, nil
}

func fetchImagesFromCollection(obj map[string]interface{}) (images []string) {
	for _, v := range obj {
		switch t := v.(type) {
		case string:
			images = append(images, t)
		case map[string]interface{}:
			images = append(images, fetchImagesFromCollection(t)...)
		}
	}
	return images
}

func setRequirementImages(osType OSType, imagesSet map[string]map[string]bool) {
	coreLabel := "core"
	switch osType {
	case Linux:
		addSourceToImage(imagesSet, settings.ShellImage.Get(), coreLabel)
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
