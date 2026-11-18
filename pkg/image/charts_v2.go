package image

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

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
	for _, versions := range index.Entries {
		version, err := c.newestVersionOfTheMinor(versions)
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
		for _, values := range versionValues {
			if err = pickImagesFromValuesMap(imagesSet, values, chartNameAndVersion, c.Config.OsType, tag); err != nil {
				return err
			}
		}
	}
	return nil
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
