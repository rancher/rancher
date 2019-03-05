package util

import (
	"strings"

	"github.com/coreos/go-semver/semver"
)

func StrToSemVer(version string) (*semver.Version, error) {
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return nil, err
	}
	return v, nil
}
