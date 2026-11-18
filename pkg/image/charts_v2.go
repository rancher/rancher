package image

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// HelmImagesAnnotationKey is the annotation of an index entry that declares the images of the chart (HIP-15).
const HelmImagesAnnotationKey = "helm.sh/images"

// annotatedImage is an entry of the helm.sh/images annotation of an index entry.
type annotatedImage struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
	// Dependency marks an image that the chart installs and its values.yaml does not have. It is the name of what the
	// chart depends on, or true, and it is not there, or false or empty, for any other image.
	Dependency interface{} `yaml:"dependency"`
}

// isDependency reports whether the image is marked as a dependency.
func (a annotatedImage) isDependency() bool {
	switch dependency := a.Dependency.(type) {
	case nil:
		return false
	case bool:
		return dependency
	case string:
		return dependency != ""
	default:
		return true
	}
}

// FetchImagesV2 is FetchImages for the catalog of the staging and production branches of rancher/charts, and it
// replaces it in GetArtifacts. FetchImages is not changed, so going back to the old system is a change of that call.
//
// The differences with FetchImages are:
//   - An index entry whose URL has a scheme other than file (oci://, https://) has no archive at that path in the
//     checkout. Its archive is at assets/<name>/<name>-<version>.tgz, which is where Rancher reads it from at
//     runtime.
//   - An archive that cannot be read for such an entry is an error. A list without the images of a chart is wrong,
//     and nothing in it tells it apart from a correct one.
//   - The version of each chart is the newest one whose catalog.cattle.io/rancher-version matches the Rancher version,
//     with no exception per chart. The index has the versions of every Rancher minor of a chart, so the newest one
//     is often for the next minor and the one that this Rancher needs is below it.
//   - The images found in the values.yaml of each chart have to be exactly the ones that its helm.sh/images annotation
//     declares, or it is an error that names the charts and the images of every difference. The list is made of what is
//     found, so a chart that declares more or less than that is not installing what the list has.
//   - The images that the annotation marks as a dependency are the exception: the chart installs them and its
//     values.yaml does not have them, which is the reason for the mark, and an air-gapped installation needs them as
//     much as the others, so they are in the list of images of Linux. The annotation does not say the system of an
//     image, so they are not in the one of Windows.
//
// Entries with a path in the repository (partner-charts, rke2-charts) are read, and skipped when the archive is not
// there, as FetchImages does. The selection of the version, and the reading of values.yaml, are the ones of
// FetchImages: if they change there, this has to follow.
func (c Charts) FetchImagesV2(imagesSet map[string]map[string]struct{}) error {
	if c.Config.ChartsPath == "" || c.Config.RancherVersion == "" {
		return nil
	}
	index, err := repo.LoadIndexFile(filepath.Join(c.Config.ChartsPath, "index.yaml"))
	if err != nil {
		return err
	}
	var problems []string
	for _, name := range sortedChartNames(index) {
		version, err := c.newestVersionOfTheMinor(index.Entries[name])
		if err != nil {
			return err
		}
		if version == nil {
			continue
		}
		archive, remote := chartArchivePathV2(version)
		versionValues, err := decodeValuesFilesInTgz(filepath.Join(c.Config.ChartsPath, archive))
		if err != nil {
			if remote {
				return errors.Wrapf(err, "failed to read the chart %s:%s", version.Name, version.Version)
			}
			logrus.Info(err)
			continue
		}
		tag := chartsToIgnoreTags[version.Name]
		chartNameAndVersion := fmt.Sprintf("%s:%s", version.Name, version.Version)
		problem, dependencies, err := compareWithAnnotation(version, versionValues, tag)
		if err != nil {
			return err
		}
		if problem != "" {
			problems = append(problems, problem)
		}
		if c.Config.OsType == Linux {
			for _, image := range dependencies {
				addSourceToImage(imagesSet, image, chartNameAndVersion)
			}
		}
		for _, values := range versionValues {
			if err = pickImagesFromValuesMap(imagesSet, values, chartNameAndVersion, c.Config.OsType, tag); err != nil {
				return err
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("the images found in the values.yaml of these charts are not the ones that their %s annotation declares:\n%s",
			HelmImagesAnnotationKey, strings.Join(problems, "\n"))
	}
	return nil
}

// sortedChartNames returns the names of the charts of the index in alphabetical order, so that what is reported is
// always in the same order.
func sortedChartNames(index *repo.IndexFile) []string {
	names := make([]string, 0, len(index.Entries))
	for name := range index.Entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// compareWithAnnotation compares the images found in the values.yaml of a chart version, for Linux and Windows, with
// the ones that its helm.sh/images annotation declares. The annotation does not tell the system of an image, so the
// images of both are compared with all of it. It returns a description of the differences, or an empty string when
// there is none, and the images that it marks as a dependency and the values.yaml does not have. A chart with no images
// and no annotation has no difference.
func compareWithAnnotation(version *repo.ChartVersion, values []map[interface{}]interface{}, tagToIgnore string) (string, []string, error) {
	source := fmt.Sprintf("%s:%s", version.Name, version.Version)

	found := map[string]struct{}{}
	for _, osType := range []OSType{Linux, Windows} {
		images := map[string]map[string]struct{}{}
		for _, v := range values {
			if err := pickImagesFromValuesMap(images, v, source, osType, tagToIgnore); err != nil {
				return "", nil, err
			}
		}
		for image := range images {
			found[image] = struct{}{}
		}
	}

	var annotated []annotatedImage
	if err := yaml.Unmarshal([]byte(version.Annotations[HelmImagesAnnotationKey]), &annotated); err != nil {
		return fmt.Sprintf("  %s\n    cannot read its %s annotation: %v", source, HelmImagesAnnotationKey, err), nil, nil
	}
	declared := map[string]struct{}{}
	declaredNotDependencies := map[string]struct{}{}
	dependencies := map[string]struct{}{}
	for _, a := range annotated {
		declared[a.Image] = struct{}{}
		if a.isDependency() {
			dependencies[a.Image] = struct{}{}
		} else {
			declaredNotDependencies[a.Image] = struct{}{}
		}
	}
	// A dependency that the chart also has is already in the list, and one that it does not is added to it.
	notFound := difference(dependencies, found)

	var lines []string
	if missing := difference(declaredNotDependencies, found); len(missing) > 0 {
		lines = append(lines, fmt.Sprintf("    declared in %s, not found in values.yaml: %s", HelmImagesAnnotationKey, strings.Join(missing, ", ")))
	}
	if undeclared := difference(found, declared); len(undeclared) > 0 {
		lines = append(lines, fmt.Sprintf("    found in values.yaml, not declared in %s: %s", HelmImagesAnnotationKey, strings.Join(undeclared, ", ")))
	}
	if len(lines) == 0 {
		return "", notFound, nil
	}
	return "  " + source + "\n" + strings.Join(lines, "\n"), notFound, nil
}

// difference returns the sorted members of a that are not in b.
func difference(a, b map[string]struct{}) []string {
	var only []string
	for image := range a {
		if _, ok := b[image]; !ok {
			only = append(only, image)
		}
	}
	sort.Strings(only)
	return only
}

// newestVersionOfTheMinor returns the newest of versions that supports the Rancher version of the export, or nil when
// none does. The versions are in the descending order that repo.LoadIndexFile gives them. A version with no
// catalog.cattle.io/rancher-version annotation supports none.
func (c Charts) newestVersionOfTheMinor(versions repo.ChartVersions) (*repo.ChartVersion, error) {
	for _, version := range versions {
		supported, err := c.checkChartVersionConstraint(*version)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check constraint of chart %s:%s", version.Name, version.Version)
		}
		if supported {
			return version, nil
		}
	}
	return nil, nil
}

// chartArchivePathV2 returns the path, relative to the root of the charts repository, of the packaged chart of
// version, and whether the index entry points outside of the repository.
func chartArchivePathV2(version *repo.ChartVersion) (string, bool) {
	if u, err := url.Parse(version.URLs[0]); err == nil && u.Scheme != "" && u.Scheme != "file" {
		return path.Join("assets", version.Name, fmt.Sprintf("%s-%s.tgz", version.Name, version.Version)), true
	}
	return version.URLs[0], false
}
