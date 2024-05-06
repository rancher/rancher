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
	libhelm "github.com/rancher/rancher/pkg/helm"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/repo"
)

const RancherVersionAnnotationKey = "catalog.cattle.io/rancher-version"

// chartsToCheckConstraints and systemChartsToCheckConstraints define which charts and system charts should
// be checked for images and added to imageSet based on whether the given Rancher version/tag satisfies the chart's
// Rancher version constraints to allow support for multiple version lines of a chart in airgap setups. If a chart is
// not defined here, only the latest version of it will be checked for images and added if it passes the constraint.
// Note: CRD charts need to be added as well.
var chartsToCheckConstraints = map[string]struct{}{
	"rancher-istio": {},
}
var systemChartsToCheckConstraints = map[string]struct{}{
	"rancher-monitoring": {},
}

// chartsToIgnoreTags and systemChartsToIgnoreTags defines the charts and system charts in which a specified
// image tag should be ignored.
var chartsToIgnoreTags = map[string]string{
	"rancher-vsphere-csi": "latest",
	"rancher-vsphere-cpi": "latest",
}
var systemChartsToIgnoreTags = map[string]string{}

type Charts struct {
	Config ExportConfig
}

// FetchImages finds all the images used by all the charts in a Rancher charts repository and adds them to imageSet.
// The images from the latest version of each chart are always added to the images set, whereas the remaining versions
// are added only if the given Rancher version/tag satisfies the chart's Rancher version constraint annotation.
func (c Charts) FetchImages(imagesSet map[string]map[string]struct{}) error {
	if c.Config.ChartsPath == "" || c.Config.RancherVersion == "" {
		return nil
	}
	index, err := repo.LoadIndexFile(filepath.Join(c.Config.ChartsPath, "index.yaml"))
	if err != nil {
		return err
	}
	// Filter index entries based on their Rancher version constraint
	var filteredVersions repo.ChartVersions
	for _, versions := range index.Entries {
		if len(versions) == 0 {
			continue
		}
		// Always append the latest version of the chart if it passes the constraint check
		// Note: Selecting the correct latest version relies on the charts-build-scripts `make standardize` command
		// sorting the versions in the index file in descending order correctly.
		latestVersion := versions[0]
		if isConstraintSatisfied, err := c.checkChartVersionConstraint(*latestVersion); err != nil {
			return errors.Wrapf(err, "failed to check constraint of chart")
		} else if isConstraintSatisfied {
			filteredVersions = append(filteredVersions, latestVersion)
		}
		// Append the remaining versions of the chart if the chart exists in the chartsToCheckConstraints map
		// and the given Rancher version satisfies the chart's Rancher version constraint annotation.
		chartName := versions[0].Metadata.Name
		if _, ok := chartsToCheckConstraints[chartName]; ok {
			for _, version := range versions[1:] {
				if isConstraintSatisfied, err := c.checkChartVersionConstraint(*version); err != nil {
					return errors.Wrapf(err, "failed to check constraint of chart")
				} else if isConstraintSatisfied {
					filteredVersions = append(filteredVersions, version)
				}
			}
		}
	}
	// Find values.yaml files in the tgz files of each chart, and check for images to add to imageSet
	for _, version := range filteredVersions {
		tgzPath := filepath.Join(c.Config.ChartsPath, version.URLs[0])
		versionValues, err := decodeValuesFilesInTgz(tgzPath)
		if err != nil {
			logrus.Info(err)
			continue
		}
		tag, _ := chartsToIgnoreTags[version.Name]
		chartNameAndVersion := fmt.Sprintf("%s:%s", version.Name, version.Version)
		for _, values := range versionValues {
			if err = pickImagesFromValuesMap(imagesSet, values, chartNameAndVersion, c.Config.OsType, tag); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkChartVersionConstraint retrieves the value of a chart's Rancher version constraint annotation, and
// returns true if the Rancher version in the export configuration satisfies the chart's constraint, false otherwise.
// If a chart does not have a Rancher version annotation defined, this function returns false.
func (c Charts) checkChartVersionConstraint(version repo.ChartVersion) (bool, error) {
	if constraintStr, ok := version.Annotations[RancherVersionAnnotationKey]; ok {
		return compareRancherVersionToConstraint(c.Config.RancherVersion, constraintStr)
	}
	return false, nil
}

type SystemCharts struct {
	Config ExportConfig
}

type Questions struct {
	RancherMinVersion string `yaml:"rancher_min_version"`
	RancherMaxVersion string `yaml:"rancher_max_version"`
}

// FetchImages finds all the images used by all the charts in a Rancher system charts repository and adds them to imageSet.
// The images from the latest version of each chart are always added to the images set, whereas the remaining versions
// are added only if the given Rancher version/tag satisfies the chart's Rancher version constraint defined in its questions file.
func (sc SystemCharts) FetchImages(imagesSet map[string]map[string]struct{}) error {
	if sc.Config.SystemChartsPath == "" || sc.Config.RancherVersion == "" {
		return nil
	}
	// Load system charts virtual index
	helm := libhelm.Helm{
		LocalPath: sc.Config.SystemChartsPath,
		IconPath:  sc.Config.SystemChartsPath,
		Hash:      "",
	}
	virtualIndex, err := helm.LoadIndex()
	if err != nil {
		return errors.Wrapf(err, "failed to load system charts index")
	}
	// Filter index entries based on their Rancher version constraint
	var filteredVersions libhelm.ChartVersions
	for _, versions := range virtualIndex.IndexFile.Entries {
		if len(versions) == 0 {
			continue
		}
		// Always append the latest version of the chart unless it has been intentionally hidden with constraints
		latestVersion := versions[0]
		if isConstraintSatisfied, err := sc.checkChartVersionConstraint(*latestVersion); err != nil {
			return errors.Wrapf(err, "failed to filter chart versions")
		} else if isConstraintSatisfied {
			filteredVersions = append(filteredVersions, latestVersion)
		}
		// Append the remaining versions of the chart if the chart exists in the systemChartsToCheckConstraints map
		// and the given Rancher version satisfies the chart's Rancher version constraint defined in its questions file
		chartName := versions[0].ChartMetadata.Name
		if _, ok := systemChartsToCheckConstraints[chartName]; ok {
			for _, version := range versions[1:] {
				if isConstraintSatisfied, err := sc.checkChartVersionConstraint(*version); err != nil {
					return errors.Wrapf(err, "failed to filter chart versions")
				} else if isConstraintSatisfied {
					filteredVersions = append(filteredVersions, version)
				}
			}
		}
	}
	// Find values.yaml files in each chart's local files, and check for images to add to imageSet
	for _, version := range filteredVersions {
		for _, file := range version.LocalFiles {
			if !isValuesFile(file) {
				continue
			}
			values, err := decodeValuesFile(file)
			if err != nil {
				return err
			}
			tag, _ := systemChartsToIgnoreTags[version.Name]
			chartNameAndVersion := fmt.Sprintf("%s:%s", version.Name, version.Version)
			if err = pickImagesFromValuesMap(imagesSet, values, chartNameAndVersion, sc.Config.OsType, tag); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkChartVersionConstraint retrieves the value of a chart's Rancher version defined in its questions file, and
// returns true if the Rancher version in the export configuration satisfies the chart's constraint, false otherwise.
// If a chart does not have a Rancher version constraint defined, this function returns false.
func (sc SystemCharts) checkChartVersionConstraint(version libhelm.ChartVersion) (bool, error) {
	questionsPath := filepath.Join(sc.Config.SystemChartsPath, version.Dir, "questions.yaml")
	questions, err := decodeQuestionsFile(questionsPath)
	if os.IsNotExist(err) {
		questionsPath = filepath.Join(sc.Config.SystemChartsPath, version.Dir, "questions.yml")
		questions, err = decodeQuestionsFile(questionsPath)
	}
	if err != nil {
		logrus.Warnf("skipping system chart, %s:%s does not have a questions file", version.ChartMetadata.Name, version.ChartMetadata.Version)
		return false, nil
	}
	constraintStr := minMaxToConstraintStr(questions.RancherMinVersion, questions.RancherMaxVersion)
	if constraintStr == "" {
		return false, nil
	}
	return compareRancherVersionToConstraint(sc.Config.RancherVersion, constraintStr)
}

// compareRancherVersionToConstraint returns true if the Rancher version satisfies constraintStr, false otherwise.
func compareRancherVersionToConstraint(rancherVersion, constraintStr string) (bool, error) {
	if constraintStr == "" {
		return false, errors.Errorf("Invalid constraint string: \"%s\"", constraintStr)
	}
	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return false, err
	}
	rancherSemVer, err := semver.NewVersion(rancherVersion)
	if err != nil {
		return false, err
	}
	// When the exporter is ran in a dev environment, we replace the rancher version with a dev version (e.g 2.X.99).
	// This breaks the semver compare logic for exporting because we use the Rancher version constraint < 2.X.99-0 in
	// many of our charts and since 2.X.99 > 2.X.99-0 the comparison returns false which is not the desired behavior.
	patch := rancherSemVer.Patch()
	if patch == 99 {
		patch = 98
	}
	// All pre-release versions are removed because the semver comparison will not yield the desired behavior unless
	// the constraint has a pre-release too. Since the exporter for charts can treat pre-releases and releases equally,
	// is cleaner to remove it. E.g. comparing rancherVersion 2.6.4-rc1 and constraint 2.6.3 - 2.6.5 yields false because
	// the versions in the contraint do not have a pre-release. This behavior comes from the semver module and is intentional.
	rSemVer, err := semver.NewVersion(fmt.Sprintf("%d.%d.%d", rancherSemVer.Major(), rancherSemVer.Minor(), patch))
	if err != nil {
		return false, err
	}
	return constraint.Check(rSemVer), nil
}

// minMaxToConstraintStr converts min and max Rancher version strings into a constraint string
// E.g min "2.6.3" max "2.6.4" -> constraintStr "2.6.3 - 2.6.4".
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

// pickImagesFromValuesMap walks a values map to find images, and add them to imagesSet.
func pickImagesFromValuesMap(imagesSet map[string]map[string]struct{}, values map[interface{}]interface{}, chartNameAndVersion string, osType OSType, tagToIgnore string) error {
	walkMap(values, func(inputMap map[interface{}]interface{}) {
		repository, ok := inputMap["repository"].(string)
		if !ok {
			return
		}
		// No string type assertion because some charts have float typed image tags
		tag, ok := inputMap["tag"]
		if !ok {
			return
		}
		if fmt.Sprintf("%v", tag) == tagToIgnore {
			return
		}
		imageName := fmt.Sprintf("%s:%v", repository, tag)
		// By default, images are added to the generic images list ("linux"). For Windows and multi-OS
		// images to be considered, they must use a comma-delineated list (e.g. "os: windows",
		// "os: windows,linux", and "os: linux,windows").
		osList, ok := inputMap["os"].(string)
		if !ok {
			if inputMap["os"] != nil {
				errors.Errorf("field 'os:' for image %s contains neither a string nor nil", imageName)
			}
			if osType == Linux {
				addSourceToImage(imagesSet, imageName, chartNameAndVersion)
				return
			}
		}
		for _, os := range strings.Split(osList, ",") {
			os = strings.TrimSpace(os)
			if strings.EqualFold("windows", os) && osType == Windows {
				addSourceToImage(imagesSet, imageName, chartNameAndVersion)
				return
			}
			if strings.EqualFold("linux", os) && osType == Linux {
				addSourceToImage(imagesSet, imageName, chartNameAndVersion)
				return
			}
		}
	})
	return nil
}

// decodeValueFilesInTgz reads tarball in tgzPath and returns a slice of values corresponding to values.yaml files found inside of it.
func decodeValuesFilesInTgz(tgzPath string) ([]map[interface{}]interface{}, error) {
	tgz, err := os.Open(tgzPath)
	if err != nil {
		return nil, err
	}
	defer tgz.Close()
	gzr, err := gzip.NewReader(tgz)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var valuesSlice []map[interface{}]interface{}
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return valuesSlice, nil
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

// walkMap walks inputMap and calls the callback function on all map type nodes including the root node.
func walkMap(inputMap interface{}, callback func(map[interface{}]interface{})) {
	switch data := inputMap.(type) {
	case map[interface{}]interface{}:
		callback(data)
		for _, value := range data {
			walkMap(value, callback)
		}
	case []interface{}:
		for _, elem := range data {
			walkMap(elem, callback)
		}
	}
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
