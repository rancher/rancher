package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"

	libhelm "github.com/rancher/rancher/pkg/helm"
	"github.com/rancher/rancher/pkg/settings"
	"gopkg.in/yaml.v2"
)

const (
	RancherVersionAnnotationKey = "catalog.cattle.io/rancher-version"
)

// ChartVersion is a wrapper type for libhelm.ChartVersion and repo.ChartVersion
type ChartVersion struct {
	*repo.ChartVersion
	Dir        string   `json:"-" yaml:"-"`
	LocalFiles []string `json:"-" yaml:"-"`
}

type ChartVersions []*ChartVersion

// ResolveCharts is the interface that wraps the methods for basic operations on charts repositories.
//
// getChartVersionsFromIndex loads the repository's index and returns all chart entries.
// filterFunc filters charts based on whether the rancher server version satisfies a given chart's rancher version constraint.
// When a chart doesn't have a constraint set, it should be added to the list of filtered charts to check it for images.
// pickImagesFromAllValues iterates through a to find values files, walk them to find images, and add them to a images set.
type ResolveCharts interface {
	getChartVersionsFromIndex() (ChartVersions, error)
	filterFunc(version ChartVersion) (bool, error)
	pickImagesFromAllValues(imagesSet map[string]map[string]bool, versions ChartVersions) error
}

// fetchImages loads rc's index to get all chart entries, filters the charts using rc's filterFunc, and then
// iterates though the slice of filtered charts to find images in their values files and add them to imagesSet.
func fetchImages(rc ResolveCharts, imagesSet map[string]map[string]bool) error {
	versions, err := rc.getChartVersionsFromIndex()
	if err != nil {
		return errors.Wrapf(err, "failed to get index")
	}
	filteredVersions, err := filterChartVersions(rc, versions)
	if err != nil {
		return errors.Wrapf(err, "failed to filter chart versions")
	}
	err = rc.pickImagesFromAllValues(imagesSet, filteredVersions)
	if err != nil {
		return errors.Wrap(err, "failed to pick images from values file")
	}
	return nil
}

// filterChartVersions takes in a slice of charts, and returns a slice of charts filtered by rc's filterFunc
func filterChartVersions(rc ResolveCharts, versions ChartVersions) (ChartVersions, error) {
	var filteredVersions ChartVersions
	for _, v := range versions {
		addToFiltered, err := rc.filterFunc(*v)
		if err != nil {
			logrus.Info(err)
			continue
		}
		if addToFiltered {
			filteredVersions = append(filteredVersions, v)
		}
	}
	return filteredVersions, nil
}

// SystemCharts implements getChartVersionsFromIndex, filterFunc, and pickImagesFromAllValues of an underlying ResolveCharts.
// It is used to fetch images of a given OS type, from charts in a system charts repository located at repoPath.
type SystemCharts struct {
	repoPath string
	osType   OSType
}

type Questions struct {
	RancherMinVersion string `yaml:"rancher_min_version"`
	RancherMaxVersion string `yaml:"rancher_max_version"`
}

// getChartVersionsFromIndex loads a virtual index created from the repository's directory, and returns all chart entries.
func (sc SystemCharts) getChartVersionsFromIndex() (ChartVersions, error) {
	if sc.repoPath == "" {
		return nil, nil
	}
	helm := libhelm.Helm{
		LocalPath: sc.repoPath,
		IconPath:  sc.repoPath,
		Hash:      "",
	}
	virtualIndex, err := helm.LoadIndex()
	if err != nil {
		return nil, err
	}
	// Convert libhelm.ChartVersion to ChartVersion wrapper type
	var versions ChartVersions
	for _, entries := range virtualIndex.IndexFile.Entries {
		for _, v := range entries {
			versions = append(versions, &ChartVersion{
				ChartVersion: &repo.ChartVersion{
					Metadata: &chart.Metadata{
						Name:    v.Name,
						Version: v.Version,
					},
				},
				Dir:        v.Dir,
				LocalFiles: v.LocalFiles,
			})
		}
	}
	return versions, nil
}

// filterFunc filters charts based on whether the rancher server version satisfies the constraint set in the chart's questions file.
// When a chart doens't have a constraint set, it is added to the list of filtered charts to check it for images.
func (sc SystemCharts) filterFunc(version ChartVersion) (bool, error) {
	questionsPath := getQuestionsPath(version.LocalFiles)
	if questionsPath == "" {
		// Log a warning and export images when a chart doesn't have a questions file
		logrus.Warnf("system chart: %s does not have a questions file", version.Name)
		return true, nil
	}
	questions, err := decodeQuestionsFile(questionsPath)
	if err != nil {
		return false, err
	}
	constraintStr := minMaxToConstraintStr(questions.RancherMinVersion, questions.RancherMaxVersion)
	if constraintStr == "" {
		// Log a warning and export images when a chart doesn't have rancher version constraints in its questions file
		logrus.Warnf("system chart: %s does not have a rancher_min_version or rancher_max_version constraint defined in its questions file", version.Name)
		return true, nil
	}
	rancherVersion := settings.GetRancherVersion()
	isInRange, err := isRancherVersionInConstraintRange(rancherVersion, constraintStr)
	if err != nil {
		return false, err
	}
	return isInRange, nil
}

// pickImagesFromAllValues iterates through a slice of charts to find images in their values files and add them to imagesSet.
func (sc SystemCharts) pickImagesFromAllValues(imagesSet map[string]map[string]bool, versions ChartVersions) error {
	for _, v := range versions {
		for _, file := range v.LocalFiles {
			if !isValuesFile(file) {
				continue
			}
			values, err := decodeValuesFile(file)
			if err != nil {
				return err
			}
			chartNameAndVersion := fmt.Sprintf("%s:%s", v.Name, v.Version)
			if err = pickImagesFromValuesMap(imagesSet, values, chartNameAndVersion, sc.osType); err != nil {
				return err
			}
		}
	}
	return nil
}

// FeatureCharts implements getChartVersionsFromIndex, filterFunc, and filterFunc of an underlying ResolveCharts, and
// It is used to fetch images of a given OS type, from charts in a feature charts repository located at repoPath.
type FeatureCharts struct {
	repoPath string
	osType   OSType
}

// getChartVersionsFromIndex loads the repository's index file, and returns all chart entries.
func (fc FeatureCharts) getChartVersionsFromIndex() (ChartVersions, error) {
	if fc.repoPath == "" {
		return nil, nil
	}
	indexPath := filepath.Join(fc.repoPath, "index.yaml")
	index, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		return nil, err
	}
	if len(index.Entries) <= 0 {
		return nil, errors.New("no entries in index file")
	}
	// Convert repo.ChartVersion to ChartVersion wrapper type
	var versions ChartVersions
	for _, entries := range index.Entries {
		for _, v := range entries {
			versions = append(versions, &ChartVersion{
				ChartVersion: v,
			})
		}
	}
	return versions, nil
}

// filterFunc filters charts based on whether the rancher server version satisfies the constraint set in the chart's rancher version annotation.
// When a chart doesn't have the annotation set, it is added to the list of filtered charts to check it for images.
func (fc FeatureCharts) filterFunc(version ChartVersion) (bool, error) {
	constraintStr, ok := version.Annotations[RancherVersionAnnotationKey]
	if !ok {
		// Log a warning when a chart doesn't have the rancher-version annotation, but return true so that images are exported.
		logrus.Warnf("feature chart: %s:%s does not have a %s annotation defined", version.Name, version.Version, RancherVersionAnnotationKey)
		return true, nil
	}
	rancherVersion := settings.GetRancherVersion()
	isInRange, err := isRancherVersionInConstraintRange(rancherVersion, constraintStr)
	if err != nil {
		return false, err
	}
	return isInRange, nil
}

// pickImagesFromAllValues iterates through a slice of filtered charts, reads their tarball to find images in their values files, and add them to imagesSet.
func (fc FeatureCharts) pickImagesFromAllValues(imagesSet map[string]map[string]bool, versions ChartVersions) error {
	for _, v := range versions {
		tgzPath := filepath.Join(fc.repoPath, v.URLs[0])
		versionTgz, err := os.Open(tgzPath)
		if err != nil {
			return err
		}
		defer versionTgz.Close()
		// Find values.yaml files in tgz
		valuesSlice, err := getDecodedValuesFromTgz(versionTgz, fc.repoPath)
		if err != nil {
			logrus.Info(err)
			continue
		}
		chartNameAndVersion := fmt.Sprintf("%s:%s", v.Name, v.Version)
		for _, values := range valuesSlice {
			// Walk values.yaml and add images to set
			if err = pickImagesFromValuesMap(imagesSet, values, chartNameAndVersion, fc.osType); err != nil {
				return err
			}
		}
	}
	return nil
}

// pickImagesFromValuesMap walks a values map to find images of a given OS type and adds them to imagesSet.
func pickImagesFromValuesMap(imagesSet map[string]map[string]bool, values map[interface{}]interface{}, chartNameAndVersion string, osType OSType) error {
	walkMap(values, func(inputMap map[interface{}]interface{}) {
		repository, ok := inputMap["repository"].(string)
		if !ok {
			return
		}
		tag, ok := inputMap["tag"].(string)
		if !ok {
			return
		}
		imageName := fmt.Sprintf("%s:%v", repository, tag)
		// By default, images are added to the generic images list ("linux"). For Windows and multi-OS
		// images to be considered, they must use a comma-delineated list (e.g. "os: windows",
		// "os: windows,linux", and "os: linux,windows").
		if osList, ok := inputMap["os"].(string); ok {
			for _, os := range strings.Split(osList, ",") {
				switch strings.TrimSpace(strings.ToLower(os)) {
				case "windows":
					if osType == Windows {
						addSourceToImage(imagesSet, imageName, chartNameAndVersion)
						return
					}
				case "linux":
					if osType == Linux {
						addSourceToImage(imagesSet, imageName, chartNameAndVersion)
						return
					}
				}
			}
		} else {
			if inputMap["os"] != nil {
				errors.Errorf("Field 'os:' for image %s contains neither a string nor nil", imageName)
			}
			if osType == Linux {
				addSourceToImage(imagesSet, imageName, chartNameAndVersion)
			}
		}
	})
	return nil
}

// walkMap walks a map and executes the given walk function for each node.
func walkMap(data interface{}, walkFunc func(map[interface{}]interface{})) {
	if inputMap, isMap := data.(map[interface{}]interface{}); isMap {
		// Run the walkFunc on the root node and each child node
		walkFunc(inputMap)
		for _, value := range inputMap {
			walkMap(value, walkFunc)
		}
	} else if inputList, isList := data.([]interface{}); isList {
		// Run the walkFunc on each element in the root node, ignoring the root itself
		for _, elem := range inputList {
			walkMap(elem, walkFunc)
		}
	}
}

// minMaxToConstraintStr converts min and max rancher version strings into a constraint string.
func minMaxToConstraintStr(min, max string) string {
	if min != "" && max != "" {
		return fmt.Sprintf("%s - %s", min, max)
	}
	if min != "" {
		return fmt.Sprintf(">= %s", min)
	}
	if max != "" {
		return fmt.Sprintf("<= %s", max)
	}
	return ""
}

// isRancherVersionInConstraintRange returns true if the rancher server version satisfies a given constraint (E.g ">=2.5.0 <=2.6"), false otherwise.
func isRancherVersionInConstraintRange(rancherVersion, constraintStr string) (bool, error) {
	if constraintStr == "" {
		return false, errors.Errorf("Invalid constraint string: \"%s\"", constraintStr)
	}
	rancherSemVer, err := semver.NewVersion(rancherVersion)
	if err != nil {
		return false, err
	}
	// Removing the pre-release because the semver package will not consider a rancherVersion with a
	// pre-releases unless the versions in the constraintStr has pre-releases as well.
	// For example: rancherVersion "2.5.7-rc1" and constraint "2.5.6 - 2.5.8" will return false because
	// there is no pre-release in the constraint "2.5.6 - 2.5.8" (This behavior is intentional).
	rancherSemVerNoPreRelease, err := rancherSemVer.SetPrerelease("")
	if err != nil {
		return false, err
	}
	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return false, err
	}
	return constraint.Check(&rancherSemVerNoPreRelease), nil
}

// getQuestionsPath iterates through chart's a slice of local files and returns a path to its questions file if one exists.
func getQuestionsPath(versionLocalFiles []string) string {
	for _, file := range versionLocalFiles {
		basename := filepath.Base(file)
		if basename == "questions.yaml" || basename == "questions.yml" {
			return file
		}
	}
	return ""
}

func decodeQuestionsFile(path string) (Questions, error) {
	var questions Questions
	file, err := os.Open(path)
	if err != nil {
		return Questions{}, err
	}
	defer file.Close()
	if err := decodeYAMLFile(file, &questions); err != nil {
		return Questions{}, err
	}
	return questions, nil
}

func decodeValuesFile(path string) (map[interface{}]interface{}, error) {
	var values map[interface{}]interface{}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := decodeYAMLFile(file, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// getDecodedValuesFromTgz reads r to find values.yaml files and returns a slice of decoded values.yaml files.
func getDecodedValuesFromTgz(r io.Reader, repoPath string) ([]map[interface{}]interface{}, error) {
	var valuesSlice []map[interface{}]interface{}
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return valuesSlice, nil
			// return any other error
		case err != nil:
			return nil, err
		case header.Typeflag == tar.TypeReg && isValuesFile(header.Name):
			var values map[interface{}]interface{}
			if err := decodeYAMLFile(tr, &values); err != nil {
				return nil, err
			}
			valuesSlice = append(valuesSlice, values)
		default:
			continue
		}
	}
}

func decodeYAMLFile(r io.Reader, target interface{}) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func isValuesFile(path string) bool {
	basename := filepath.Base(path)
	return basename == "values.yaml" || basename == "values.yml"
}
