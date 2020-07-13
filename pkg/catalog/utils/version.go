package utils

import (
	"sort"
	"strings"

	"github.com/blang/semver"
	mVersion "github.com/mcuadros/go-version"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/catalog/utils/version"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
)

func VersionBetween(a, b, c string) bool {
	if a == "" && c == "" {
		return true
	} else if a == "" {
		return !VersionGreaterThan(b, c)
	} else if b == "" {
		return true
	} else if c == "" {
		return !VersionGreaterThan(a, b)
	}
	return !VersionGreaterThan(a, b) && !VersionGreaterThan(b, c)
}

func formatVersion(v, rng string) (string, string) {

	v = strings.TrimLeft(v, "v")

	rng = strings.TrimLeft(rng, "v")
	rng = strings.Replace(rng, ">v", ">", -1)
	rng = strings.Replace(rng, ">=v", ">=", -1)
	rng = strings.Replace(rng, "<v", "<", -1)
	rng = strings.Replace(rng, "<=v", "<=", -1)
	rng = strings.Replace(rng, "=v", "=", -1)
	rng = strings.Replace(rng, "!v", "!", -1)

	return v, rng
}

func VersionSatisfiesRange(v, rng string) (bool, error) {

	v, rng = formatVersion(v, rng)

	sv, err := semver.Parse(v)
	if err != nil {
		return false, err
	}

	rangeFunc, err := semver.ParseRange(rng)
	if err != nil {
		return false, err
	}
	return rangeFunc(sv), nil
}

func VersionGreaterThan(a, b string) bool {
	return version.GreaterThan(a, b)
}

func ValidateRancherVersion(template *v3.CatalogTemplateVersion) error {
	rancherMin := template.Spec.RancherMinVersion
	rancherMax := template.Spec.RancherMaxVersion

	serverVersion := settings.ServerVersion.Get()

	// don't compare if we are running as dev or in the build env
	if !ReleaseServerVersion(serverVersion) {
		return nil
	}

	if rancherMin != "" && !mVersion.Compare(serverVersion, rancherMin, ">=") {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "rancher min version not met")
	}

	if rancherMax != "" && !mVersion.Compare(serverVersion, rancherMax, "<=") {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "rancher max version exceeded")
	}

	return nil
}

func ReleaseServerVersion(serverVersion string) bool {
	if serverVersion == "dev" ||
		serverVersion == "master" ||
		serverVersion == "" ||
		strings.HasSuffix(serverVersion, "-head") {
		return false
	}
	return true
}

func LatestAvailableTemplateVersion(template *v3.CatalogTemplate) (*v3.TemplateVersionSpec, error) {
	versions := template.DeepCopy().Spec.Versions
	if len(versions) == 0 {
		return nil, errors.New("empty catalog template version list")
	}

	sort.Slice(versions, func(i, j int) bool {
		val1, err := semver.ParseTolerant(versions[i].Version)
		if err != nil {
			return false
		}

		val2, err := semver.ParseTolerant(versions[j].Version)
		if err != nil {
			return false
		}

		return val2.LT(val1)
	})

	for _, templateVersion := range versions {
		catalogTemplateVersion := &v3.CatalogTemplateVersion{
			TemplateVersion: v3.TemplateVersion{
				Spec: templateVersion,
			},
		}
		if err := ValidateRancherVersion(catalogTemplateVersion); err == nil {
			return &templateVersion, nil
		}
	}

	return nil, errors.Errorf("template %s allowed rancher version not match current server", template.Name)
}
