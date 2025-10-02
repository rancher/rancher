package semver

import (
	"github.com/Masterminds/semver/v3"
)

type Version string

func (v Version) IsDev() bool {
	if v == "dev" {
		return true
	}

	semVersion, err := semver.NewVersion(string(v))
	return err != nil || // When version is not SemVer it is dev
		semVersion.Prerelease() != "" // When the version includes pre-release assume dev
}
