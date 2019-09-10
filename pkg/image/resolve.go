package image

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rancher/norman/types/convert"
	libhelm "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	img "github.com/rancher/types/image"
	"gopkg.in/yaml.v2"
)

var requiredImagesNotInSystemCharts = []string{
	"busybox",
}

func Resolve(image string) string {
	reg := settings.SystemDefaultRegistry.Get()
	if reg != "" && !strings.HasPrefix(image, reg) {
		//Images from Dockerhub Library repo, we add rancher prefix when using private registry
		if !strings.Contains(image, "/") {
			image = "rancher/" + image
		}
		return path.Join(reg, image)
	}

	return image
}

func CollectionImages(objs ...interface{}) ([]string, error) {
	images := map[string]bool{}

	for _, obj := range objs {
		data := map[string]interface{}{}
		if err := convert.ToObj(obj, &data); err != nil {
			return nil, err
		}
		findStrings(data, images)
	}

	var result []string
	for k := range images {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result, nil
}

func GetLinuxImages(systemChartPath string, imagesFromArgs []string, rkeSystemImages map[string]v3.RKESystemImages) ([]string, error) {
	images, err := getImagesFromCharts(systemChartPath)
	if err != nil {
		return []string{}, err
	}

	if len(imagesFromArgs) > 0 {
		images = append(images, imagesFromArgs...)
	}
	images = append(images, requiredImagesNotInSystemCharts...)
	targetImages, err := CollectionImages(rkeSystemImages, v3.ToolsSystemImages)
	if err != nil {
		return []string{}, err
	}
	for _, i := range images {
		targetImages = append(targetImages, img.Mirror(i))
	}
	return targetImages, nil
}

func GetWindowsImages(rkeSystemImages map[string]v3.RKESystemImages) ([]string, error) {
	images, err := CollectionImages(rkeSystemImages)
	if err != nil {
		return []string{}, err
	}
	return images, nil
}

func findStrings(obj map[string]interface{}, found map[string]bool) {
	for _, v := range obj {
		switch t := v.(type) {
		case string:
			found[t] = true
		case map[string]interface{}:
			findStrings(t, found)
		}
	}
}

func getImagesFromCharts(path string) ([]string, error) {
	var images []string
	imageMap := map[string]struct{}{}
	chartVersion, err := getChartAndVersion(path)
	if err != nil {
		return nil, err
	}
	if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		return walkFunc(imageMap, chartVersion, path, p, info, err)
	}); err != nil {
		return images, err
	}
	for value := range imageMap {
		images = append(images, value)
	}
	return images, nil
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

func walkFunc(images map[string]struct{}, versions map[string]string, basePath, path string, info os.FileInfo, err error) error {
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
		generateImages(inputMap, images)
	})
	return nil
}

func generateImages(inputMap map[interface{}]interface{}, output map[string]struct{}) {
	r, repoOk := inputMap["repository"]
	t, tagOk := inputMap["tag"]
	if !repoOk || !tagOk {
		return
	}
	repo, repoOk := r.(string)
	if !repoOk {
		return
	}

	output[fmt.Sprintf("%s:%v", repo, t)] = struct{}{}

	return
}

func walkthroughMap(inputMap map[interface{}]interface{}, walkFunc func(map[interface{}]interface{})) {
	walkFunc(inputMap)
	for _, value := range inputMap {
		if v, ok := value.(map[interface{}]interface{}); ok {
			walkthroughMap(v, walkFunc)
		}
	}
}
