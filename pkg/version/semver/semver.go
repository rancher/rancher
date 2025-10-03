package semver

import (
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	releasePattern       = regexp.MustCompile("^v[0-9]")
	branchReleasePattern = regexp.MustCompile("^v[0-9].x")
)

type Version string

// HasReleasePrefix validates the value against the "legacy" releasePattern
// It is not necessarily a valid SemVer, but does have the expected "release prefix".
func (v Version) HasReleasePrefix() bool {
	return releasePattern.MatchString(string(v))
}

// HasBranchPrefix validates the value as having a minor version set to `x`
func (v Version) HasBranchPrefix() bool {
	return branchReleasePattern.MatchString(string(v))
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

// IsRC validates the version is SemVer with a "Prerelease" matching the "RC" variety
func (v Version) IsRC() bool {
	semVersion, err := semver.NewVersion(string(v))
	if err != nil {
		return false
	}
	return semVersion.Prerelease() != "" &&
		strings.HasPrefix(strings.ToLower(semVersion.Prerelease()), "rc")
}
