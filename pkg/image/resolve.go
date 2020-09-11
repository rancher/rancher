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

func getChartAndVersion(path string) (map[string]string, error) {
	rtn := map[string]string{}
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
			rtn[k] = versions[0].Dir
		}
	}

	return rtn, nil
}

func pickImagesFromValuesYAML(imagesSet map[string]struct{}, versions map[string]string, basePath, path string, info os.FileInfo, osType OSType) error {
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return err
	}
	var found bool
	for _, v := range versions {
		if strings.HasPrefix(relPath, v) {
			found = true
			break
		}
	}
	if !found || info.Name() != "values.yaml" {
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
		generateImages(inputMap, imagesSet, osType)
	})
	return nil
}

func generateImages(inputMap map[interface{}]interface{}, output map[string]struct{}, osType OSType) {
	r, repoOk := inputMap["repository"]
	t, tagOk := inputMap["tag"]
	if !repoOk || !tagOk {
		return
	}
	repo, repoOk := r.(string)
	if !repoOk {
		return
	}

	// distinguish images by os
	os := inputMap["os"]
	switch os {
	case "windows": // must have indicate `os: windows` if the image is using in Windows cluster
		if osType != Windows {
			return
		}
	default:
		if osType != Linux {
			return
		}
	}

	output[fmt.Sprintf("%s:%v", repo, t)] = struct{}{}
}

func walkthroughMap(inputMap map[interface{}]interface{}, walkFunc func(map[interface{}]interface{})) {
	walkFunc(inputMap)
	for _, value := range inputMap {
		if v, ok := value.(map[interface{}]interface{}); ok {
			walkthroughMap(v, walkFunc)
		}
	}
}

func GetImages(systemChartPath, chartPath string, k3sUpgradeImages, imagesFromArgs []string, rkeSystemImages map[string]rketypes.RKESystemImages, osType OSType) ([]string, error) {
	var images []string

	// fetch images from system charts
	if systemChartPath != "" {
		imagesInSystemCharts, err := fetchImagesFromCharts(systemChartPath, osType)
		if err != nil {
			return []string{}, errors.Wrap(err, "failed to fetch images from system charts")
		}
		images = append(images, imagesInSystemCharts...)
	}

	// fetch images from charts
	if chartPath != "" {
		imagesInCharts, err := fetchImagesFromCharts(chartPath, osType)
		if err != nil {
			return []string{}, errors.Wrap(err, "failed to fetch images from charts")
		}
		images = append(images, imagesInCharts...)
	}

	// fetch images from system images
	if len(rkeSystemImages) > 0 {
		imagesInSystem, err := fetchImagesFromSystem(rkeSystemImages, osType)
		if err != nil {
			return []string{}, errors.Wrap(err, "failed to fetch images from system images")
		}
		images = append(images, imagesInSystem...)
	}

	// append images from requirement
	if requirementImages := getRequirementImages(osType); len(requirementImages) > 0 {
		images = append(images, requirementImages...)
	}

	// append images from args
	if len(imagesFromArgs) > 0 {
		images = append(images, imagesFromArgs...)
	}

	// append images for k3s-upgrade
	if len(k3sUpgradeImages) > 0 {
		images = append(images, k3sUpgradeImages...)
	}

	return normalizeImages(images), nil
}

func fetchImagesFromCharts(path string, osType OSType) ([]string, error) {
	chartVersion, err := getChartAndVersion(path)
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to get chart and version from %q", path)
	}

	imagesSet := map[string]struct{}{}
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return pickImagesFromValuesYAML(imagesSet, chartVersion, path, p, info, osType)
	})
	if err != nil {
		return []string{}, errors.Wrap(err, "failed to pick images from values.yaml")
	}

	var images []string
	for image := range imagesSet {
		images = append(images, image)
	}
	return images, nil
}

func fetchImagesFromSystem(rkeSystemImages map[string]rketypes.RKESystemImages, osType OSType) ([]string, error) {
	collectionImagesList := []interface{}{
		rkeSystemImages,
	}
	switch osType {
	case Linux:
		collectionImagesList = append(collectionImagesList, v32.ToolsSystemImages)
	}

	return flatImagesFromCollections(collectionImagesList...)
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

func getRequirementImages(osType OSType) []string {
	switch osType {
	case Linux:
		return []string{
			"busybox",
			settings.ShellImage.Get(),
		}
	}
	return []string{}
}

func normalizeImages(rawImages []string) []string {
	var images []string

	// mirror
	for i := range rawImages {
		rawImages[i] = img.Mirror(rawImages[i])
	}

	// unique
	var imagesSet = map[string]bool{}
	for _, image := range rawImages {
		if _, exist := imagesSet[image]; !exist {
			imagesSet[image] = true
			images = append(images, image)
		}
	}

	// sort
	sort.Slice(images, func(i, j int) bool {
		return images[i] < images[j]
	})

	return images
}
