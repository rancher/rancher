package image

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

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

type chart struct {
	dir     string
	version string
}

const (
	Linux OSType = iota
	Windows
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

func getChartAndVersion(path string) (map[string]chart, error) {
	rtn := map[string]chart{}
	helm := libhelm.Helm{
		LocalPath: path,
		IconPath:  path,
		Hash:      "",
	}
	index, err := helm.LoadIndex()
	if err != nil {
		return nil, err
	}
	for k, versions := range index.IndexFile.Entries {
		// because versions is sorted in reverse order, the first one will be the latest version
		if len(versions) > 0 {
			newestVersionedChart := versions[0]
			rtn[k] = chart{
				dir:     newestVersionedChart.Dir,
				version: newestVersionedChart.Version}
		}
	}

	return rtn, nil
}

func pickImagesFromValuesYAML(imagesSet map[string]map[string]bool, charts map[string]chart, basePath, path string, info os.FileInfo, osType OSType) error {
	if info.Name() != "values.yaml" {
		return nil
	}
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return err
	}
	var chartNameAndVersion string
	for name, chart := range charts {
		if strings.HasPrefix(relPath, chart.dir) {
			chartNameAndVersion = fmt.Sprintf("%s:%s", name, chart.version)
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
	dataInterface := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(data, &dataInterface); err != nil {
		return err
	}

	walkthroughMap(dataInterface, func(inputMap map[interface{}]interface{}) {
		generateImages(chartNameAndVersion, inputMap, imagesSet, osType)
	})
	return nil
}

func generateImages(chartNameAndVersion string, inputMap map[interface{}]interface{}, output map[string]map[string]bool, osType OSType) {
	r, repoOk := inputMap["repository"]
	t, tagOk := inputMap["tag"]
	if !repoOk || !tagOk {
		return
	}
	repo, repoOk := r.(string)
	if !repoOk {
		return
	}

	imageName := fmt.Sprintf("%s:%v", repo, t)

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

func walkthroughMap(data interface{}, walkFunc func(map[interface{}]interface{})) {
	if inputMap, isMap := data.(map[interface{}]interface{}); isMap {
		// Run the walkFunc on the root node and each child node
		walkFunc(inputMap)
		for _, value := range inputMap {
			walkthroughMap(value, walkFunc)
		}
	} else if inputList, isList := data.([]interface{}); isList {
		// Run the walkFunc on each element in the root node, ignoring the root itself
		for _, elem := range inputList {
			walkthroughMap(elem, walkFunc)
		}
	}
}

func GetImages(systemChartPath, chartPath string, k3sUpgradeImages, imagesFromArgs []string, rkeSystemImages map[string]rketypes.RKESystemImages, osType OSType) ([]string, []string, error) {
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

func fetchImagesFromCharts(path string, osType OSType, imagesSet map[string]map[string]bool) error {
	chartVersion, err := getChartAndVersion(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart and version from %q", path)
	}

	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return pickImagesFromValuesYAML(imagesSet, chartVersion, path, p, info, osType)
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
