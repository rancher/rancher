package semver

import (
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	releasePattern = regexp.MustCompile("^v[0-9]")
)

type Version string

// HasReleasePrefix validates the value against the "legacy" releasePattern
// It is not necessarily a valid SemVer, but does have the expected "release prefix".
func (v Version) HasReleasePrefix() bool {
	return releasePattern.MatchString(string(v))
}

// IsDevOrPrerelease reports if the version is a non-stable, dev, or pre-release build.
func (v Version) IsDevOrPrerelease() bool {
	if v == "dev" {
		return true
	}

	semVersion, err := semver.NewVersion(string(v))
	return err != nil || // When version is not SemVer it is dev
		semVersion.Prerelease() != "" // When the version includes pre-release assume dev
}

// IsRC validates the version is SemVer with a "Prerelease" matching only the "RC" variety
//
// This helper was added to replicate existing behaviour that treats "RC" builds as "pre-stable".
// Whereas other releases with Prerelease value included are treated as "unstable".
// This is used by the content catalog to ensure that RC builds have the same catalog logic as a release.
func (v Version) IsRC() bool {
	semVersion, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return semVersion.Prerelease() != "" &&
		strings.HasPrefix(strings.ToLower(semVersion.Prerelease()), "rc")
}
