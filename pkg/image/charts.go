package image

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	libhelm "github.com/rancher/rancher/pkg/helm"
	"gopkg.in/yaml.v2"
)

type chart struct {
	dir     string
	version string
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
